// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// AllocationInput is the income pool, plan and spending in base-currency minor units.
type AllocationInput struct {
	Income   int64
	Rollover int64
	Budgeted int64
	Savings  int64
	Spent    int64
}

// Allocation describes the plan state and bar geometry on one shared money axis.
type Allocation struct {
	Pool       int64 // income (+ rolled-over leftover) the plan is measured against
	Budgeted   int64
	Savings    int64
	Unassigned int64 // negative when the plan runs past the pool
	PlanPct    int   // assigned as a percent of the pool; may exceed 100
	State      string

	BudgetedPct int // bar: fill up to the income tick
	OverPct     int // bar: striped fill past the income tick
	SavingsPct  int
	GapPct      int
	MarkerPct   int // -1 when there is nothing past the tick to mark

	// Spending, measured on the SAME money axis as the segments above. Zero-based
	// has three quantities that nest — income holds the plan, the plan holds the
	// spending — and nesting cannot be read off two charts with different
	// denominators. Plotting spend against this scale lets the rail be compared to
	// the plan and the income tick by LENGTH, with no percentages to reconcile.
	Spent    int64
	SpentPct int
}

// ResolveAllocation computes the allocation state and integer bar percentages.
// It reports unrepresentable monetary totals rather than returning wrapped values.
func ResolveAllocation(v AllocationInput) (Allocation, error) {
	pool, err := money.Add(v.Income, v.Rollover)
	if err != nil {
		return Allocation{}, fmt.Errorf("add income and rollover: %w", err)
	}
	r := Allocation{
		Pool:      pool,
		Budgeted:  v.Budgeted,
		Savings:   v.Savings,
		MarkerPct: -1,
	}
	if r.Pool <= 0 {
		return r, nil
	}
	assigned, err := money.Add(r.Budgeted, r.Savings)
	if err != nil {
		return Allocation{}, fmt.Errorf("sum allocated budget amounts: %w", err)
	}
	r.Unassigned, err = money.Sub(r.Pool, assigned)
	if err != nil {
		return Allocation{}, fmt.Errorf("calculate unassigned income: %w", err)
	}
	planPct, err := money.MulDiv(assigned, 100, r.Pool, money.RoundTowardZero)
	if err != nil || int64(int(planPct)) != planPct {
		if err == nil {
			err = fmt.Errorf("plan percentage does not fit int")
		}
		return Allocation{}, fmt.Errorf("calculate allocation percentage: %w", err)
	}
	r.PlanPct = int(planPct)
	switch {
	case assigned == 0:
		r.State = "empty" // an income basis with no budgets yet — day one
	case r.Unassigned < 0:
		r.State = "over"
	case r.Unassigned == 0:
		r.State = "exact"
	default:
		r.State = "under"
	}

	// Widths are a share of the LARGER of the pool or what is assigned, so an
	// over-assigned plan still fits the track and the overshoot stays visible
	// rather than being clipped at 100%.
	scale := r.Pool
	if assigned > scale {
		scale = assigned
	}
	gap := r.Unassigned
	if gap < 0 {
		gap = 0
	}
	pctOf := func(n int64) (int, error) {
		if scale <= 0 || n <= 0 {
			return 0, nil
		}
		p, err := money.MulDiv(n, 100, scale, money.RoundTowardZero)
		if err != nil {
			return 0, err
		}
		if p < 100 {
			return int(p), nil
		}
		return 100, nil
	}
	r.Spent = v.Spent
	r.SpentPct, err = pctOf(r.Spent)
	if err != nil {
		return Allocation{}, fmt.Errorf("calculate spending percentage: %w", err)
	}
	r.BudgetedPct, err = pctOf(r.Budgeted)
	if err != nil {
		return Allocation{}, fmt.Errorf("calculate budgeted percentage: %w", err)
	}
	r.GapPct, err = pctOf(gap)
	if err != nil {
		return Allocation{}, fmt.Errorf("calculate income gap percentage: %w", err)
	}
	r.SavingsPct = 100 - r.BudgetedPct - r.GapPct // the remainder, so the bar fills exactly
	if r.SavingsPct < 0 {
		r.SavingsPct = 0
	}
	// The income tick only means something when the plan runs past it. When the
	// plan is under, the END of the track already IS the pool, so a tick at 100%
	// would mark a boundary the track edge already draws.
	if r.State == "over" {
		markerPct, err := money.MulDiv(r.Pool, 100, scale, money.RoundTowardZero)
		if err != nil || int64(int(markerPct)) != markerPct {
			if err == nil {
				err = fmt.Errorf("income marker percentage does not fit int")
			}
			return Allocation{}, fmt.Errorf("calculate income marker: %w", err)
		}
		r.MarkerPct = int(markerPct)
		// Everything past the tick is money that does not exist, and it must not
		// wear the healthy accent. The first draft did, and a bar that is entirely
		// green while its caption says "more than you earn" argues with itself —
		// the reader believes the colour.
		if r.BudgetedPct > r.MarkerPct {
			r.OverPct = r.BudgetedPct - r.MarkerPct
			r.BudgetedPct = r.MarkerPct
		}
	}
	return r, nil
}
