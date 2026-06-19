package reports

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// SpendingByWeekday buckets expenses by day of the week over the half-open period
// [start, end), in base-currency minor units. The result is indexed by
// time.Weekday (Sunday = 0 … Saturday = 6), so callers can read totals[t.Weekday]
// directly. Transfers and income are excluded (IsExpense). This surfaces a
// behavioral pattern — which days money tends to leave.
func SpendingByWeekday(txns []domain.Transaction, start, end time.Time, rates currency.Rates) ([7]int64, error) {
	var totals [7]int64
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return [7]int64{}, err
		}
		totals[t.Date.Weekday()] += conv.Abs().Amount
	}
	return totals, nil
}

// PeakWeekday returns the weekday with the most spend and whether there was any
// spend at all (ok is false when every day is zero). Ties resolve to the earliest
// weekday (Sunday first) for determinism.
func PeakWeekday(totals [7]int64) (time.Weekday, bool) {
	peak, any := time.Sunday, false
	var best int64
	for d := time.Sunday; d <= time.Saturday; d++ {
		if v := totals[d]; v > best {
			best, peak, any = v, d, true
		}
	}
	return peak, any
}
