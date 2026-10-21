// SPDX-License-Identifier: MIT

package engineenv

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// budgetObservation retains the established formula-unit calculations while
// distinguishing a measured zero from unavailable spending. Limit is independent
// of spending; every other value requires SpendingKnown.
type budgetObservation struct {
	Limit, Spent, Remaining, Over, Percent float64
	SpendingKnown                          bool
	Err                                    error
}

func observeBudget(b domain.Budget, d Data, major func(int64) float64) budgetObservation {
	observation := budgetObservation{Limit: major(b.Limit.Amount)}
	start, end := budgeting.PeriodRange(b.Period, d.Now, d.WeekStart)
	spent, err := budgeting.Spent(b, d.budgetLedger(), start, end, d.Rates)
	if err != nil {
		observation.Err = fmt.Errorf("observe budget %q spending: %w", b.ID, err)
		return observation
	}
	observation.SpendingKnown = true
	observation.Spent = major(spent.Amount)
	observation.Remaining = observation.Limit - observation.Spent
	if observation.Spent > observation.Limit {
		observation.Over = observation.Spent - observation.Limit
	}
	if observation.Limit != 0 {
		observation.Percent = observation.Spent / observation.Limit * 100
	}
	return observation
}
