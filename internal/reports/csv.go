// SPDX-License-Identifier: MIT

package reports

import (
	"bytes"
	"encoding/csv"
	"strconv"
)

// CategoryCSV renders a spending-by-category report as CSV bytes: a header row
// then one row per category with its name, this-period amount, prior-period
// amount, and percent change (blank when there's no comparison baseline). It is
// decoupled from formatting via callbacks — name resolves a category id to a
// label and amount renders a minor-units value as a plain decimal string (no
// currency symbol or grouping, so the numbers stay spreadsheet-friendly). Pure:
// it builds the bytes with the standard library, no platform dependencies.
func CategoryCSV(rows []CategorySpend, name func(id string) string, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Category", "Amount", "Prior", "Change %"})
	for _, r := range rows {
		change := ""
		if r.HasDelta {
			change = strconv.FormatInt(r.DeltaPct, 10)
		}
		_ = w.Write([]string{name(r.CategoryID), amount(r.Amount), amount(r.Prior), change})
	}
	w.Flush()
	return buf.Bytes()
}

// MemberCSV renders a spending-by-member report as CSV bytes: a header row then
// one row per member with the resolved name and amount. Like CategoryCSV it is
// decoupled from formatting via callbacks (name resolves a member id, amount
// renders minor units as a plain decimal). Pure, standard-library only.
func MemberCSV(rows []MemberSpend, name func(id string) string, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Member", "Amount"})
	for _, r := range rows {
		_ = w.Write([]string{name(r.MemberID), amount(r.Amount)})
	}
	w.Flush()
	return buf.Bytes()
}

// CustomFieldCSV renders a spending-by-custom-field report as CSV bytes: a
// header row then one row per distinct field value with the display value and
// total amount. fieldLabel is the column header for the value column (typically
// the field's Label from its Def). amount renders a minor-units integer as a
// plain decimal string. Pure, standard-library only.
func CustomFieldCSV(rows []CustomFieldSpend, fieldLabel string, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{fieldLabel, "Amount"})
	for _, r := range rows {
		_ = w.Write([]string{r.Value, amount(r.Amount)})
	}
	w.Flush()
	return buf.Bytes()
}

// PayeeCSV renders a top-payees report as CSV bytes: a header row then one row
// per payee with its name and total spend. amount renders minor-units as a plain
// decimal string. Pure, standard-library only.
func PayeeCSV(rows []PayeeTotal, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Payee", "Amount"})
	for _, r := range rows {
		name := r.Name
		if name == "" {
			name = "(no description)"
		}
		_ = w.Write([]string{name, amount(r.Amount)})
	}
	w.Flush()
	return buf.Bytes()
}

// LargestExpensesCSV renders a largest-expenses report as CSV bytes: a header
// row then one row per item with its description, category id (the caller may
// pre-resolve names by wrapping the amount callback if needed, but a simple
// approach passes the raw id and lets the spreadsheet user join it), date
// (ISO yyyy-mm-dd), and amount. amount renders minor-units as a plain decimal.
// Pure, standard-library only.
func LargestExpensesCSV(rows []ExpenseItem, name func(id string) string, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Description", "Category", "Date", "Amount"})
	for _, r := range rows {
		desc := r.Desc
		if desc == "" {
			desc = "(no description)"
		}
		_ = w.Write([]string{desc, name(r.CategoryID), r.Date.Format("2006-01-02"), amount(r.Amount)})
	}
	w.Flush()
	return buf.Bytes()
}

// YearTaxCSV renders a year-end / tax summary (L16) as CSV: a per-category block
// (Category, Income, Expense, Net) followed by a TOTAL row with the headline
// income/expense/net. name resolves a category id to a label; amount renders
// minor units as a plain decimal. Pure, standard-library only.
func YearTaxCSV(s YearTaxSummary, name func(id string) string, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Tax year", strconv.Itoa(s.Year)})
	_ = w.Write([]string{"Category", "Income", "Expense", "Net"})
	for _, r := range s.Rows {
		_ = w.Write([]string{name(r.CategoryID), amount(r.Income), amount(r.Expense), amount(r.Net)})
	}
	_ = w.Write([]string{"TOTAL", amount(s.TotalIncome), amount(s.TotalExpense), amount(s.NetIncome)})
	w.Flush()
	return buf.Bytes()
}
