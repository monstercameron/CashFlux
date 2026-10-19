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

func receiptCurrency(r extract.Receipt) (string, error) {
	currencyCode := strings.ToUpper(strings.TrimSpace(r.Currency))
	for _, amount := range receiptAmounts(r) {
		for symbol, code := range map[string]string{"€": "EUR", "£": "GBP", "¥": "JPY"} {
			if !strings.Contains(amount, symbol) {
				continue
			}
			if currencyCode != "" && currencyCode != code {
				return "", fmt.Errorf("appstate: receipt currency %q conflicts with its %s amount", currencyCode, symbol)
			}
			currencyCode = code
		}
	}
	return currencyCode, nil
}

func receiptAmounts(r extract.Receipt) []string {
	amounts := make([]string, 0, len(r.Lines)+1)
	amounts = append(amounts, r.Total)
	for _, line := range r.Lines {
		amounts = append(amounts, line.Amount)
	}
	return amounts
}

func accountReceiptCurrency(a *App, accountID string) (string, error) {
	account, ok := findAccount(a, accountID)
	if !ok {
		return "", fmt.Errorf("appstate: account %q not found", accountID)
	}
	code := strings.ToUpper(strings.TrimSpace(account.Currency))
	if code == "" {
		return "", fmt.Errorf("appstate: account %q has no currency", accountID)
	}
	return code, nil
}

func validateReceiptCurrency(r extract.Receipt, accountCurrency string) error {
	code, err := receiptCurrency(r)
	if err != nil {
		return err
	}
	if code != "" && code != accountCurrency {
		return fmt.Errorf("appstate: receipt currency %s does not match account currency %s", code, accountCurrency)
	}
	return nil
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
	accountCurrency, err := accountReceiptCurrency(a, accountID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if err := validateReceiptCurrency(r, accountCurrency); err != nil {
		return domain.Transaction{}, err
	}
	dec := currency.Decimals(accountCurrency)
	if !r.Reconciles(dec) {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt line splits do not sum to the total")
	}
	totalMinor, err := r.TotalMinor(dec)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt total: %w", err)
	}
	negativeTotal, err := money.Neg(totalMinor)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt total sign: %w", err)
	}

	cats := a.Categories()
	userRules := a.Rules()
	splits := make([]domain.CategorySplit, 0, len(r.Lines))
	for _, line := range r.Lines {
		lineMinor, err := line.AmountMinor(dec)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("appstate: receipt line %q: %w", line.Description, err)
		}
		negativeLine, err := money.Neg(lineMinor)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("appstate: receipt line %q sign: %w", line.Description, err)
		}
		splits = append(splits, domain.CategorySplit{
			CategoryID: a.mapReceiptCategory(cats, userRules, line, r.Merchant),
			Amount:     money.New(negativeLine, accountCurrency), // an expense is negative
		})
	}

	desc := strings.TrimSpace(r.Merchant)
	if desc == "" {
		desc = "Receipt"
	}
	tx := domain.Transaction{
		ID: id.New(), AccountID: accountID, Date: date, Desc: desc, Payee: strings.TrimSpace(r.Merchant),
		Amount: money.New(negativeTotal, accountCurrency), Splits: splits, Source: domain.TxnSourceScanned,
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

// AttachReceiptToTransaction documents an EXISTING transaction with a scanned
// receipt (#58) instead of creating a duplicate row: the receipt's lines become
// the transaction's category splits (validated against its amount), a blank
// payee takes the merchant, and the row is marked reviewed — the charge was
// just confirmed against paper. The transaction's amount must equal the
// receipt total exactly; a mismatched attach would silently rewrite money.
func (a *App) AttachReceiptToTransaction(txnID string, r extract.Receipt) (domain.Transaction, error) {
	var tx domain.Transaction
	found := false
	for _, t := range a.Transactions() {
		if t.ID == txnID {
			tx, found = t, true
			break
		}
	}
	if !found {
		return domain.Transaction{}, fmt.Errorf("appstate: transaction %q not found", txnID)
	}
	accountCurrency, err := accountReceiptCurrency(a, tx.AccountID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if strings.ToUpper(strings.TrimSpace(tx.Amount.Currency)) != accountCurrency {
		return domain.Transaction{}, fmt.Errorf("appstate: transaction currency %s does not match account currency %s", tx.Amount.Currency, accountCurrency)
	}
	if err := validateReceiptCurrency(r, accountCurrency); err != nil {
		return domain.Transaction{}, err
	}
	dec := currency.Decimals(accountCurrency)
	if !r.Reconciles(dec) {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt line splits do not sum to the total")
	}
	totalMinor, err := r.TotalMinor(dec)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt total: %w", err)
	}
	negativeTotal, err := money.Neg(totalMinor)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt total sign: %w", err)
	}
	if tx.Amount.Amount != negativeTotal || strings.ToUpper(strings.TrimSpace(tx.Amount.Currency)) != accountCurrency {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt total %d %s does not match the transaction amount %d %s", negativeTotal, accountCurrency, tx.Amount.Amount, tx.Amount.Currency)
	}

	cats := a.Categories()
	userRules := a.Rules()
	splits := make([]domain.CategorySplit, 0, len(r.Lines))
	for _, line := range r.Lines {
		lineMinor, lerr := line.AmountMinor(dec)
		if lerr != nil {
			return domain.Transaction{}, fmt.Errorf("appstate: receipt line %q: %w", line.Description, lerr)
		}
		negativeLine, lerr := money.Neg(lineMinor)
		if lerr != nil {
			return domain.Transaction{}, fmt.Errorf("appstate: receipt line %q sign: %w", line.Description, lerr)
		}
		splits = append(splits, domain.CategorySplit{
			CategoryID: a.mapReceiptCategory(cats, userRules, line, r.Merchant),
			Amount:     money.New(negativeLine, accountCurrency),
		})
	}
	tx.Splits = splits
	if cat := singleCategory(splits); cat != "" {
		tx.CategoryID = cat
	}
	if strings.TrimSpace(tx.Payee) == "" {
		tx.Payee = strings.TrimSpace(r.Merchant)
	}
	tx.Reviewed = true
	if !tx.SplitsReconcile() {
		return domain.Transaction{}, fmt.Errorf("appstate: receipt splits do not reconcile to the transaction amount")
	}
	if err := a.PutTransaction(tx); err != nil {
		return domain.Transaction{}, err
	}
	a.log.Info("receipt attached", "id", tx.ID, "lines", len(splits), "total", tx.Amount.String())
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
