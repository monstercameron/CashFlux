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
