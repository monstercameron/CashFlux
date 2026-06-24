// SPDX-License-Identifier: MIT

package extract

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/money"
)

// ReceiptLine is one categorized portion of a single receipt charge.
type ReceiptLine struct {
	Description string
	Category    string
	Amount      string
}

// Receipt is a single bank/card charge split across categories: a total plus the
// categorized lines that should sum to it. This is the key distinction from a
// statement — a statement is many charges (import as many transactions), whereas a
// receipt is ONE charge (import as one transaction split across categories), so it
// neither double-counts against the single charge the bank reports nor breaks
// dedupe against it.
type Receipt struct {
	Date     string
	Merchant string
	Total    string
	Lines    []ReceiptLine
}

// ReceiptFromRows builds a Receipt from extracted vision rows, treating each row
// as a categorized line. When totalHint is empty the total defaults to the sum of
// the lines (formatted to decimals), so a receipt with no printed total still
// reconciles by construction.
func ReceiptFromRows(rows []Row, date, merchant, totalHint string, decimals int) Receipt {
	lines := make([]ReceiptLine, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, ReceiptLine{Description: r.Description, Category: r.Category, Amount: r.Amount})
	}
	total := strings.TrimSpace(totalHint)
	if total == "" {
		if sum, err := linesTotalMinor(lines, decimals); err == nil {
			total = money.FormatMinor(sum, decimals)
		}
	}
	return Receipt{Date: date, Merchant: merchant, Total: total, Lines: lines}
}

// LinesTotalMinor sums the receipt's line amounts in minor units.
func (r Receipt) LinesTotalMinor(decimals int) (int64, error) {
	return linesTotalMinor(r.Lines, decimals)
}

// TotalMinor parses the receipt's total to integer minor units (tolerating the
// $/comma formatting a model emits).
func (r Receipt) TotalMinor(decimals int) (int64, error) {
	return parseAmountMinor(r.Total, decimals)
}

// AmountMinor parses a line's amount to integer minor units.
func (l ReceiptLine) AmountMinor(decimals int) (int64, error) {
	return parseAmountMinor(l.Amount, decimals)
}

// Residual returns total minus the sum of the lines, in minor units: 0 means the
// splits reconcile to the total to the cent, a positive value means the lines fall
// short of the total (unassigned remainder), and a negative value means they
// overshoot it. Any unparsable amount is an error.
func (r Receipt) Residual(decimals int) (int64, error) {
	total, err := parseAmountMinor(r.Total, decimals)
	if err != nil {
		return 0, err
	}
	sum, err := linesTotalMinor(r.Lines, decimals)
	if err != nil {
		return 0, err
	}
	return total - sum, nil
}

// Reconciles reports whether the line splits sum exactly to the total.
func (r Receipt) Reconciles(decimals int) bool {
	resid, err := r.Residual(decimals)
	return err == nil && resid == 0
}

func linesTotalMinor(lines []ReceiptLine, decimals int) (int64, error) {
	var sum int64
	for _, l := range lines {
		amt, err := parseAmountMinor(l.Amount, decimals)
		if err != nil {
			return 0, err
		}
		sum += amt
	}
	return sum, nil
}

// parseAmountMinor tolerates the currency symbols and grouping commas a vision
// model often emits ("$1,234.50") before parsing to integer minor units.
func parseAmountMinor(s string, decimals int) (int64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "$", "")
	s = strings.ReplaceAll(s, ",", "")
	return money.ParseMinor(s, decimals)
}
