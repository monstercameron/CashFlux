package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// SpendStats summarizes the individual expenses in a period: how many, their
// total, the mean, and the median (which resists the skew a single large
// purchase puts on the mean). All amounts are base-currency minor units.
type SpendStats struct {
	Count   int
	Total   int64
	Average int64 // Total / Count, 0 when there are no expenses
	Median  int64 // middle expense magnitude (mean of the two middle for an even count)
}

// SpendingStats computes per-transaction expense statistics over the half-open
// period [start, end) in the base currency. Transfers and income are excluded;
// amounts convert through the FX table. With no expenses it returns a zero value.
func SpendingStats(txns []domain.Transaction, start, end time.Time, rates currency.Rates) (SpendStats, error) {
	var amounts []int64
	var total int64
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount.Abs(), rates.Base)
		if err != nil {
			return SpendStats{}, err
		}
		amounts = append(amounts, conv.Amount)
		total += conv.Amount
	}
	n := len(amounts)
	if n == 0 {
		return SpendStats{}, nil
	}
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] < amounts[j] })
	var median int64
	if n%2 == 1 {
		median = amounts[n/2]
	} else {
		median = (amounts[n/2-1] + amounts[n/2]) / 2
	}
	return SpendStats{Count: n, Total: total, Average: total / int64(n), Median: median}, nil
}
