// SPDX-License-Identifier: MIT

package importdraft

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importaudit"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/receiptmatch"
)

// ReviewCurrency uses the selected account, household base, then USD.
func ReviewCurrency(accounts []domain.Account, accountID, base string) string {
	if account, ok := domain.AccountByID(accounts, accountID); ok && account.Currency != "" {
		return account.Currency
	}
	if base == "" {
		return "USD"
	}
	return base
}

// AccountName resolves an account without substituting another account's name.
func AccountName(accounts []domain.Account, id string) string {
	account, _ := domain.AccountByID(accounts, id)
	return account.Name
}

// AmountPresentation keeps unreadable source text and otherwise formats its sign.
func AmountPresentation(raw, cur string, color bool, format func(money.Money) string) (text, class string, kind domain.CategoryKind) {
	text, class, kind = raw, "amount fig", domain.KindExpense
	minor, err := money.ParseMinor(strings.TrimSpace(raw), currency.Decimals(cur))
	if err != nil {
		return
	}
	if cur != "" {
		text = format(money.New(minor, cur))
	}
	if minor > 0 {
		kind = domain.KindIncome
	}
	if color {
		class += SignClass(minor)
	}
	return
}

// SignClass selects the existing amount tone, including its separating space.
func SignClass(minor int64) string {
	if minor > 0 {
		return " amount-income"
	}
	if minor < 0 {
		return " amount-expense"
	}
	return ""
}

// MagnitudeText displays the unsigned magnitude even at the int64 lower bound.
// Accounting formatting handles the magnitude without negating MinInt64.
func MagnitudeText(minor int64, cur string) string {
	text := money.FormatAccounting(minor, currency.Decimals(cur), currency.Symbol(cur))
	if minor < 0 {
		return strings.TrimSuffix(strings.TrimPrefix(text, "("), ")")
	}
	return text
}

// ReceiptReview contains the checked reconciliation facts for a reviewed receipt.
type ReceiptReview struct {
	Currency                         string
	Residual                         int64
	Reconciled, Unreadable, Disabled bool
}

// ReconcileReceipt checks signed lines without overflowing the residual magnitude.
func ReconcileReceipt(rows []extract.Row, total, cur string, errors int, saving bool) ReceiptReview {
	cur = ReviewCurrency(nil, "", cur)
	r := ReceiptReview{Currency: cur}
	residual, err := Receipt(rows, "", total).Residual(currency.Decimals(cur))
	if err == nil {
		r.Residual, err = money.Abs(residual)
	}
	r.Unreadable = err != nil
	r.Reconciled = err == nil && residual == 0
	r.Disabled = !r.Reconciled || errors > 0 || saving
	return r
}

// StatementDisabled prevents invalid, overflowing, empty or pending commits.
func StatementDisabled(summary Summary, errors int, saving bool) bool {
	return saving || summary.Invalid > 0 || summary.Overflow || errors > 0 || summary.Importable == 0
}

// RecentDocuments returns at most five newest imports, preserving equal-time order.
// The input slice is never sorted or shortened in place.
func RecentDocuments(docs []domain.Document) []domain.Document {
	out := append([]domain.Document(nil), docs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].UploadedAt.After(out[j].UploadedAt) })
	return out[:min(len(out), 5)]
}

// HistoryText builds the existing import kind and honest ledger reconciliation copy.
func HistoryText(d domain.Document, accountName string, txns []domain.Transaction, translate Translate) (label, result, when string) {
	key := "documents.historyKindCSV"
	switch d.Kind {
	case "pdf":
		key = "imb.historyPDF"
	case "statement":
		key = "imb.historyStatement"
	case "mixed":
		key = "imb.historyMixed"
	case domain.DocImage:
		key = "documents.historyKindDoc"
	}
	label = translate(key)
	if accountName != "" {
		label += " · " + accountName
	}
	imported := d.RowCount
	if imported == 0 {
		imported = len(d.Extracted)
	}
	tally := importaudit.For(d, txns)
	result = translate("documents.historyImported", translate(CountKey(imported, "transaction"), imported))
	if tally.Split() {
		if tally.Duplicates > 0 {
			result += " " + translate("documents.historyDuplicates", tally.Duplicates)
		}
		if tally.Failed > 0 {
			result += " " + translate("documents.historyFailed", tally.Failed)
		}
	} else if d.SkippedCount > 0 {
		result += " " + translate("documents.historySkipped", d.SkippedCount)
	}
	switch {
	case !tally.ActiveKnown && imported > 0:
		result += " " + translate("documents.historyUntraceable")
	case tally.Removed > 0:
		result += " " + translate("documents.historyRemoved", tally.Active, tally.Removed)
	}
	return label, result, d.UploadedAt.Format("Jan 2, 2006 3:04 PM")
}

// MatchReason describes the existing receipt candidate match without changing order.
func MatchReason(candidate receiptmatch.Candidate, translate Translate) string {
	why := translate("documents.receiptMatchSameDay")
	if candidate.DaysApart > 0 {
		why = translate("documents.receiptMatchDaysApart", candidate.DaysApart)
	}
	if candidate.MerchantHit {
		why += " · " + translate("documents.receiptMatchMerchant")
	}
	return why
}

// StatusText applies the import region's existing status priority.
func StatusText(message, failure string, loading, saving, pending bool, translate Translate) string {
	text := message
	if text == "" {
		text = translate("imb.ready")
	}
	if failure != "" {
		text = failure
	}
	if loading {
		text = translate("documents.reading")
	}
	if saving && !pending {
		text = translate("enc34.saving")
	}
	return text
}

// LearnedCategories returns each nonempty split category once, in source order.
func LearnedCategories(splits []domain.CategorySplit) []string {
	var out []string
	seen := map[string]bool{}
	for _, split := range splits {
		if split.CategoryID != "" && !seen[split.CategoryID] {
			seen[split.CategoryID] = true
			out = append(out, split.CategoryID)
		}
	}
	return out
}
