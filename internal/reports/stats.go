// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
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
// amounts convert through the FX table. With no expenses it returns a zero
// value. Unrepresentable magnitudes or totals return an error with no partial
// statistics.
func SpendingStats(txns []domain.Transaction, start, end time.Time, rates currency.Rates) (SpendStats, error) {
	var amounts []int64
	var total int64
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		magnitude, err := t.Amount.AbsChecked()
		if err != nil {
			return SpendStats{}, fmt.Errorf("spending statistics: transaction %q magnitude: %w", t.ID, err)
		}
		conv, err := rates.Convert(magnitude, rates.Base)
		if err != nil {
			return SpendStats{}, fmt.Errorf("spending statistics: transaction %q conversion: %w", t.ID, err)
		}
		amounts = append(amounts, conv.Amount)
		total, err = money.Add(total, conv.Amount)
		if err != nil {
			return SpendStats{}, fmt.Errorf("spending statistics total at transaction %q: %w", t.ID, err)
		}
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
		// Both values are nonnegative and sorted. This midpoint never forms
		// an overflowing sum, and retains integer truncation of the old median.
		lower, upper := amounts[n/2-1], amounts[n/2]
		median = lower + (upper-lower)/2
	}
	return SpendStats{Count: n, Total: total, Average: total / int64(n), Median: median}, nil
}
