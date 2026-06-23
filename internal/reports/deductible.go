package reports

import (
	"bytes"
	"encoding/csv"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// DeductibleRow is one deductible category's expense total for the reporting
// period, in base-currency minor units.  CategoryID is the domain.Category.ID;
// callers resolve it to a display name.
type DeductibleRow struct {
	CategoryID string
	Amount     int64 // absolute expense in base-currency minor units
}

// DeductibleSummary is the tax-deductible spending report: per-category rows
// for every deductible-flagged category that had expense in [start, end), plus
// a headline total.
type DeductibleSummary struct {
	Rows  []DeductibleRow
	Total int64
}

// DeductibleTotals sums expense transactions dated in [start, end) for each
// category whose Deductible flag is true.  Transfers and income are excluded;
// foreign amounts convert through rates.  Rows are sorted by largest amount
// first (ties broken by category id for determinism).
func DeductibleTotals(
	txns []domain.Transaction,
	cats []domain.Category,
	start, end time.Time,
	rates currency.Rates,
) (DeductibleSummary, error) {
	// Build a set of deductible category ids for O(1) lookup.
	deductible := make(map[string]bool, len(cats))
	for _, c := range cats {
		if c.Deductible {
			deductible[c.ID] = true
		}
	}

	totals := map[string]int64{}
	for _, t := range txns {
		if !t.IsExpense() {
			continue
		}
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		if !deductible[t.CategoryID] {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return DeductibleSummary{}, err
		}
		totals[t.CategoryID] += conv.Abs().Amount
	}

	rows := make([]DeductibleRow, 0, len(totals))
	var grand int64
	for id, amt := range totals {
		rows = append(rows, DeductibleRow{CategoryID: id, Amount: amt})
		grand += amt
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Amount != rows[j].Amount {
			return rows[i].Amount > rows[j].Amount
		}
		return rows[i].CategoryID < rows[j].CategoryID
	})
	return DeductibleSummary{Rows: rows, Total: grand}, nil
}

// DeductibleCSV renders a deductible-totals report as CSV bytes: a header row
// then one row per deductible category with its name and amount, plus a TOTAL
// row at the end.  name resolves a category id to a label; amount renders
// minor-unit integers as plain decimal strings.  Pure, standard-library only.
func DeductibleCSV(s DeductibleSummary, name func(id string) string, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Category", "Deductible Expense"})
	for _, r := range s.Rows {
		_ = w.Write([]string{name(r.CategoryID), amount(r.Amount)})
	}
	_ = w.Write([]string{"TOTAL", amount(s.Total)})
	w.Flush()
	return buf.Bytes()
}
