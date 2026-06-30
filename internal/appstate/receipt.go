// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// mapReceiptCategory resolves a receipt line to an existing category id. The
// extracted per-line category is the primary signal, so it is matched by name
// first (exact, then fuzzy); when the line has no category that names a real one,
// it falls back to the user's auto-categorization rules on the line description +
// merchant (so "Costco -> Groceries" still applies). "" when nothing matches.
func (a *App) mapReceiptCategory(cats []domain.Category, userRules []rules.Rule, line extract.ReceiptLine, merchant string) string {
	if cid := resolveCategoryName(cats, line.Category); cid != "" {
		return cid
	}
	if r := rules.FirstMatch(userRules, strings.TrimSpace(line.Description+" "+merchant)); r != nil {
		return r.SetCategoryID
	}
	return ""
}

// resolveCategoryName maps a free-text category name to an existing expense
// category id: an exact (case-insensitive) name match, else a fuzzy match where the
// name is a substring either way ("Grocery" <-> "Groceries"). "" when none fit.
func resolveCategoryName(cats []domain.Category, name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	for _, c := range cats {
		if c.Kind == domain.KindExpense && strings.ToLower(c.Name) == name {
			return c.ID
		}
	}
	for _, c := range cats {
		if c.Kind != domain.KindExpense {
			continue
		}
		cn := strings.ToLower(c.Name)
		if cn != "" && (strings.Contains(cn, name) || strings.Contains(name, cn)) {
			return c.ID
		}
	}
	return ""
}

// ImportReceipt turns a reconciled receipt into ONE expense transaction carrying a
// category split per line — so it counts once against the account yet reports
// per-category spend. Each line's free-text category is mapped to a real category
// (via the auto-rules), amounts are stored as negative (an expense), and the splits
// are validated to sum to the transaction amount before persisting. It returns the
// created transaction.
func (a *App) ImportReceipt(r extract.Receipt, accountID string, date time.Time) (domain.Transaction, error) {
	if accountID == "" {
		return domain.Transaction{}, fmt.Errorf("appstate: a receipt needs an account to import into")
	}
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	if !r.Reconciles(dec) {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt line splits do not sum to the total")
	}
	totalMinor, err := r.TotalMinor(dec)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt total: %w", err)
	}

	cats := a.Categories()
	userRules := a.Rules()
	splits := make([]domain.CategorySplit, 0, len(r.Lines))
	for _, line := range r.Lines {
		lineMinor, err := line.AmountMinor(dec)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("appstate: receipt line %q: %w", line.Description, err)
		}
		splits = append(splits, domain.CategorySplit{
			CategoryID: a.mapReceiptCategory(cats, userRules, line, r.Merchant),
			Amount:     money.New(-lineMinor, base), // an expense is negative
		})
	}

	desc := strings.TrimSpace(r.Merchant)
	if desc == "" {
		desc = "Receipt"
	}
	tx := domain.Transaction{
		ID: id.New(), AccountID: accountID, Date: date, Desc: desc, Payee: strings.TrimSpace(r.Merchant),
		Amount: money.New(-totalMinor, base), Splits: splits, Source: domain.TxnSourceScanned,
	}
	// A single-category receipt also gets that category on the transaction itself.
	if cat := singleCategory(splits); cat != "" {
		tx.CategoryID = cat
	}
	if !tx.SplitsReconcile() {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt splits do not reconcile to the total")
	}
	if err := a.PutTransaction(tx); err != nil {
		return domain.Transaction{}, err
	}
	a.log.Info("receipt imported", "id", tx.ID, "lines", len(splits), "total", tx.Amount.String())
	return tx, nil
}

// singleCategory returns the shared category id when every split carries the same
// non-empty category, else "".
func singleCategory(splits []domain.CategorySplit) string {
	cat := ""
	for _, s := range splits {
		if s.CategoryID == "" {
			return ""
		}
		if cat == "" {
			cat = s.CategoryID
		} else if cat != s.CategoryID {
			return ""
		}
	}
	return cat
}
