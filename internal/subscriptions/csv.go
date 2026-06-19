package subscriptions

import (
	"bytes"
	"encoding/csv"
)

// CSV renders the detected subscriptions as CSV bytes: a header row then one row
// per subscription with its name, cadence, charge, normalized monthly and annual
// cost, and next renewal date (ISO yyyy-mm-dd). Money values are rendered by the
// amount callback as plain decimals (no currency symbol or grouping) so the file
// stays spreadsheet-friendly. Pure: built with the standard library.
func CSV(subs []Subscription, amount func(int64) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Name", "Cadence", "Charge", "Monthly", "Annual", "Next renewal"})
	for _, s := range subs {
		_ = w.Write([]string{
			s.Name,
			string(s.Cadence),
			amount(s.Amount),
			amount(s.MonthlyAmount()),
			amount(s.AnnualAmount()),
			s.NextRenewal.Format("2006-01-02"),
		})
	}
	w.Flush()
	return buf.Bytes()
}
