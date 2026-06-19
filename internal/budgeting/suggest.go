package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// SuggestLimit proposes a monthly budget limit for a category from recent
// history: the average monthly spend in that category over the trailing `months`
// full months ending before the current month (the current partial month is
// excluded so it doesn't understate the figure). Amounts convert to the base
// currency; transfers and income are excluded.
//
// The average is taken over the span from the oldest month with spend through the
// most recent — so a brand-new category (one month of data) isn't diluted by
// empty leading months, while genuine zero-spend months within that span still
// count (they pull the average down, as a budget should reflect). Returns 0 when
// the category is empty, months is non-positive, or there's no spend to learn
// from.
func SuggestLimit(categoryID string, txns []domain.Transaction, now time.Time, months int, rates currency.Rates) (int64, error) {
	if categoryID == "" || months <= 0 {
		return 0, nil
	}
	curStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	monthSums := make([]int64, months+1) // monthSums[k] = spend in the k-th prior full month
	for k := 1; k <= months; k++ {
		ms := dateutil.AddMonths(curStart, -k)
		me := dateutil.AddMonths(curStart, -(k - 1))
		for _, t := range txns {
			if !t.IsExpense() || t.CategoryID != categoryID || !dateutil.InRange(t.Date, ms, me) {
				continue
			}
			conv, err := rates.Convert(t.Amount.Abs(), rates.Base)
			if err != nil {
				return 0, err
			}
			monthSums[k] += conv.Amount
		}
	}

	oldest := 0 // largest k that had any spend → span end
	var total int64
	for k := 1; k <= months; k++ {
		total += monthSums[k]
		if monthSums[k] > 0 {
			oldest = k
		}
	}
	if oldest == 0 {
		return 0, nil
	}
	return total / int64(oldest), nil
}
