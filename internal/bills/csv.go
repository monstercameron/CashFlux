// SPDX-License-Identifier: MIT

package bills

import (
	"bytes"
	"encoding/csv"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/money"
)

// CSV renders upcoming bills as CSV bytes: a header row then one row per bill
// with its name, due date (ISO yyyy-mm-dd), days until due, and amount. The
// amount callback formats each bill's payment (so the caller can convert to a
// base currency and render a plain, spreadsheet-friendly decimal). Pure: built
// with the standard library.
func CSV(bs []Bill, amount func(money.Money) string) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Name", "Due date", "Days until", "Amount"})
	for _, b := range bs {
		_ = w.Write([]string{
			b.Name,
			b.DueDate.Format("2006-01-02"),
			strconv.Itoa(b.DaysUntil),
			amount(b.Amount),
		})
	}
	w.Flush()
	return buf.Bytes()
}
