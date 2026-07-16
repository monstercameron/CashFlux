// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// ExpenseItem is one individual expense in a largest-expenses report, in base-
// currency minor units (positive magnitude).
type ExpenseItem struct {
	Desc       string
	CategoryID string
	Amount     int64
	Date       time.Time
}

// LargestExpenses returns the biggest individual expenses over [start, end),
// largest first — "what were my biggest purchases this period". Transfers and
// income are excluded and amounts are converted to the base currency. n <= 0
// returns them all; otherwise the top n. Ties on amount are broken by most
// recent date, then description, for a deterministic order.
func LargestExpenses(txns []domain.Transaction, start, end time.Time, rates currency.Rates, n int) ([]ExpenseItem, error) {
	var out []ExpenseItem
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		out = append(out, ExpenseItem{
			Desc:       t.Desc,
			CategoryID: t.CategoryID,
			Amount:     conv.Abs().Amount,
			Date:       t.Date,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.After(out[j].Date)
		}
		return out[i].Desc < out[j].Desc
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}
