// SPDX-License-Identifier: MIT

package budgetview

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
)

// PoolInput carries the budget plan facts used by the summary pool indicators.
type PoolInput struct {
	Method    budgeting.Methodology
	Expected  int64
	Rollover  int64
	Budgeted  int64
	Fixed     int64
	PlanKnown bool
}

// PoolPreparation is the checked, method-specific unallocated pool. Amount is
// meaningful only when Known and HasPool are both true; zero is then a known
// empty pool. Envelope and other non-pool methods are known to have no pool.
type PoolPreparation struct {
	Known   bool
	Amount  int64
	HasPool bool
	Error   error
}

// PreparePool calculates the unallocated amount shown by the budget summary.
// Arithmetic failures remain explicit instead of wrapping into a misleading
// pool amount or a false over-assignment decision.
func PreparePool(in PoolInput) PoolPreparation {
	switch in.Method {
	case budgeting.MethodZeroBased:
		if !in.PlanKnown {
			return PoolPreparation{Error: fmt.Errorf("zero-based pool: plan is unavailable")}
		}
		pool, err := money.Add(in.Expected, in.Rollover)
		if err != nil {
			return PoolPreparation{Error: fmt.Errorf("sum income and rollover: %w", err)}
		}
		assigned, err := money.Add(in.Budgeted, in.Fixed)
		if err != nil {
			return PoolPreparation{Error: fmt.Errorf("sum budget and fixed assignments: %w", err)}
		}
		amount, err := money.Sub(pool, assigned)
		if err != nil {
			return PoolPreparation{Error: fmt.Errorf("calculate zero-based pool: %w", err)}
		}
		return PoolPreparation{Known: true, Amount: amount, HasPool: true}
	case budgeting.MethodSimple:
		if !in.PlanKnown {
			return PoolPreparation{Error: fmt.Errorf("simple pool: plan is unavailable")}
		}
		amount, err := money.Sub(in.Expected, in.Budgeted)
		if err != nil {
			return PoolPreparation{Error: fmt.Errorf("calculate simple pool: %w", err)}
		}
		return PoolPreparation{Known: true, Amount: amount, HasPool: true}
	default:
		return PoolPreparation{Known: true}
	}
}
