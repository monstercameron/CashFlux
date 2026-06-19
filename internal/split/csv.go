package split

import (
	"bytes"
	"encoding/csv"
)

// CSV renders settle-up transfers as CSV bytes: a header row then one row per
// transfer with the resolved payer/payee names and the amount. It is decoupled
// from formatting via callbacks — name resolves a member id to a label and amount
// renders minor units as a plain decimal string (no symbol or grouping, so the
// numbers stay spreadsheet-friendly). Pure: built with the standard library, no
// platform dependencies.
func CSV(transfers []Transfer, name func(id string) string, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"From", "To", "Amount"})
	for _, tr := range transfers {
		_ = w.Write([]string{name(tr.From), name(tr.To), amount(tr.Amount)})
	}
	w.Flush()
	return buf.Bytes()
}
