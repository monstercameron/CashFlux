package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// YearTaxRow is one category's annual income and expense totals for the year-end
// summary — the per-line figures you hand a tax preparer or drop into a return.
// Both Income and Expense are positive base-currency minor units; Net is
// Income - Expense (negative when the category cost more than it brought in).
// CategoryID is empty for uncategorized activity; the caller resolves names.
type YearTaxRow struct {
	CategoryID string
	Income     int64
	Expense    int64
	Net        int64
}

// YearTaxSummary is a whole tax year's income and expense rolled up by category
// with headline totals — the "Year-end / tax summary" report (L16). Year is the
// label for the report header; the actual transactions counted are those dated in
// the half-open period [start, end) passed to YearTax, so callers can report a
// calendar year or a fiscal year by choosing the bounds.
type YearTaxSummary struct {
	Year         int
	Rows         []YearTaxRow
	TotalIncome  int64
	TotalExpense int64
	NetIncome    int64 // TotalIncome - TotalExpense
}

// YearTax rolls every income and expense transaction dated in [start, end) up by
// category, in the base currency, for a year-end / tax summary. Transfers are
// excluded (IsIncome / IsExpense already skip them); foreign amounts convert
// through the FX table like the rest of the reports. Rows are sorted by largest
// net magnitude first (the categories that move the return most), ties broken by
// category id for determinism. year is echoed into the summary as the header
// label and does not itself filter — pass the bounds that define the tax year.
func YearTax(txns []domain.Transaction, year int, start, end time.Time, rates currency.Rates) (YearTaxSummary, error) {
	income := map[string]int64{}
	expense := map[string]int64{}
	for _, t := range txns {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		switch {
		case t.IsIncome():
			conv, err := rates.Convert(t.Amount, rates.Base)
			if err != nil {
				return YearTaxSummary{}, err
			}
			income[t.CategoryID] += conv.Abs().Amount
		case t.IsExpense():
			conv, err := rates.Convert(t.Amount, rates.Base)
			if err != nil {
				return YearTaxSummary{}, err
			}
			expense[t.CategoryID] += conv.Abs().Amount
		}
	}

	ids := make(map[string]struct{}, len(income)+len(expense))
	for id := range income {
		ids[id] = struct{}{}
	}
	for id := range expense {
		ids[id] = struct{}{}
	}

	sum := YearTaxSummary{Year: year, Rows: make([]YearTaxRow, 0, len(ids))}
	for id := range ids {
		row := YearTaxRow{CategoryID: id, Income: income[id], Expense: expense[id]}
		row.Net = row.Income - row.Expense
		sum.Rows = append(sum.Rows, row)
		sum.TotalIncome += row.Income
		sum.TotalExpense += row.Expense
	}
	sum.NetIncome = sum.TotalIncome - sum.TotalExpense

	sort.Slice(sum.Rows, func(i, j int) bool {
		ai, aj := absI64(sum.Rows[i].Net), absI64(sum.Rows[j].Net)
		if ai != aj {
			return ai > aj
		}
		return sum.Rows[i].CategoryID < sum.Rows[j].CategoryID
	})
	return sum, nil
}

// absI64 returns the absolute value of a signed minor-unit amount.
func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
