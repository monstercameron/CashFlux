// SPDX-License-Identifier: MIT

package reports

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// SavingsRateSeries returns the savings rate (whole percent of income kept) for
// each consecutive bucket defined by bounds — bucket i is [bounds[i],
// bounds[i+1]) — oldest first, for a savings-rate trend. It reuses
// IncomeExpenseSeries, so the rate matches the dashboard's (0 when a bucket had
// no income, negative when spending exceeded income). Fewer than two bounds
// yields an empty result.
func SavingsRateSeries(txns []domain.Transaction, bounds []time.Time, rates currency.Rates) ([]int, error) {
	flows, err := IncomeExpenseSeries(txns, bounds, rates)
	if err != nil {
		return nil, err
	}
	out := make([]int, len(flows))
	for i, f := range flows {
		out[i] = f.SavingsRate()
	}
	return out, nil
}
