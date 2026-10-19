// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// This file holds the pure "leftover sweep" logic (XC6): at a month boundary,
// compute how much each participating budget left unspent and total it into a
// single amount that one approval can earmark toward a goal. It is platform-
// independent and table-driven-tested; the state/UI layers own persistence, the
// month-boundary prompt, and the write into goal earmarks.

// SweepConfig is the household's leftover-sweep policy: which budgets contribute
// their unspent remainder and which goal receives the swept total. It is the
// persisted configuration behind the month-close ritual.
type SweepConfig struct {
	// Enabled turns the month-close sweep ritual on.
	Enabled bool
	// BudgetIDs are the budgets that participate — only their leftovers are swept.
	BudgetIDs []string
	// TargetGoalID is the goal the swept total is earmarked toward.
	TargetGoalID string
}

// Participates reports whether a budget is in the sweep set.
func (c SweepConfig) Participates(budgetID string) bool {
	for _, id := range c.BudgetIDs {
		if id == budgetID {
			return true
		}
	}
	return false
}

// SuppressesRollover reports whether the sweep policy overrides a budget's own
// rollover setting. Sweep and rollover are mutually exclusive per budget, and
// sweep wins when both are enabled: a participating budget's leftover is swept to
// the goal instead of carried into next period. Callers advancing rollover math
// should skip Carryover for a budget this returns true for.
func (c SweepConfig) SuppressesRollover(budgetID string) bool {
	return c.Enabled && c.Participates(budgetID)
}

// SweepLine is one budget's contribution to a sweep: the positive amount it left
// unspent this period.
type SweepLine struct {
	// BudgetID identifies the contributing budget.
	BudgetID string
	// BudgetName is the budget's display name (for the card copy).
	BudgetName string
	// Leftover is the budget's unspent remainder for the period (always positive;
	// overspent budgets contribute nothing).
	Leftover money.Money
}

// SweepPlan is the computed result of a month-close sweep: the per-budget lines
// and their total, plus the goal the total is destined for. A plan with no lines
// (Total zero) means nothing to sweep — the card should not appear.
type SweepPlan struct {
	// Error marks an incomplete plan; a failed plan must never move money.
	Error error
	// Lines are the participating budgets that left money unspent.
	Lines []SweepLine
	// Total is the sum of all line leftovers, in the base currency.
	Total money.Money
	// GoalID is the target goal the total earmarks toward (from the config).
	GoalID string
	// Blocked is true when the sweep is withheld because the target goal fails the
	// earmark-integrity gate (XC7) — its linked account is already over-earmarked.
	// A blocked plan keeps its Lines/Total for display but must not be applied.
	Blocked bool
}

// HasLeftover reports whether the plan has anything worth sweeping.
func (p SweepPlan) HasLeftover() bool {
	return p.Error == nil && p.Total.Amount > 0 && len(p.Lines) > 0
}

// BudgetCount is the number of budgets contributing to the sweep.
func (p SweepPlan) BudgetCount() int {
	return len(p.Lines)
}

// ComputeSweep is the single-currency compatibility wrapper. Foreign currency,
// overflow or a failed status produces a blocked plan with Error. Use
// ComputeSweepWithRates for household FX.
func ComputeSweep(statuses []Status, cfg SweepConfig, base string, goalAllowed func(string) bool) SweepPlan {
	plan, err := ComputeSweepWithRates(statuses, cfg, currency.Rates{Base: base}, goalAllowed)
	if err != nil {
		plan.Error = err
		plan.Blocked = true
	}
	return plan
}

// ComputeSweepWithRates computes checked leftovers from evaluated card statuses.
// Month-close sweeps only monthly spending budgets; weekly, quarterly, yearly
// and saving budgets are excluded. Every line and Total use rates.Base.
// Any eligible failure withholds the entire plan instead of moving a subtotal.
func ComputeSweepWithRates(statuses []Status, cfg SweepConfig, rates currency.Rates, goalAllowed func(string) bool) (SweepPlan, error) {
	plan := SweepPlan{Total: money.Zero(rates.Base), GoalID: cfg.TargetGoalID}
	fail := func(err error) (SweepPlan, error) {
		return SweepPlan{GoalID: cfg.TargetGoalID, Blocked: true, Error: err}, err
	}
	if !cfg.Enabled {
		return plan, nil
	}
	if rates.Base == "" {
		return fail(fmt.Errorf("sweep: base currency is required"))
	}
	for _, s := range statuses {
		b := s.Budget
		// Rollover already gives this remainder to the next period. Sweeping it
		// as well would count the same dollars twice (once for the goal and once
		// as carry-in). Sweep and per-budget rollover are mutually exclusive.
		if !cfg.Participates(b.ID) || b.IsSaving() || b.Rollover || b.Period != domain.PeriodMonthly {
			continue
		}
		if s.Error != nil {
			return fail(s.Error)
		}
		if s.State == StateUnavailable {
			return fail(fmt.Errorf("sweep budget %q is unavailable", b.ID))
		}
		if s.Remaining.Amount <= 0 {
			continue
		}
		amount, err := rates.Convert(s.Remaining, rates.Base)
		if err != nil {
			return fail(fmt.Errorf("sweep budget %q: %w", b.ID, err))
		}
		plan.Total, err = plan.Total.Add(amount)
		if err != nil {
			return fail(fmt.Errorf("sum sweep leftovers: %w", err))
		}
		plan.Lines = append(plan.Lines, SweepLine{BudgetID: b.ID, BudgetName: b.Name, Leftover: amount})
	}
	if goalAllowed != nil && cfg.TargetGoalID != "" && !goalAllowed(cfg.TargetGoalID) {
		plan.Blocked = true
	}
	return plan, nil
}

// EvaluateSweep evaluates the closed month through the same carry, boost,
// descendant and smoothing model as cards, retaining the active member lens.
func EvaluateSweep(budgets []domain.Budget, in ViewInput, cfg SweepConfig, memberID string, goalAllowed func(string) bool) (SweepPlan, error) {
	// Pooled rollover carries prior-period slack from every eligible budget.
	// A sweep over that same remainder would allocate it a second time.
	if in.PoolRollover {
		return SweepPlan{Total: money.Zero(in.Rates.Base), GoalID: cfg.TargetGoalID}, nil
	}
	var statuses []Status
	for _, b := range ViewBudgets(budgets, memberID) {
		if !cfg.Participates(b.ID) || b.Period != domain.PeriodMonthly || b.IsSaving() || b.Rollover {
			continue
		}
		statuses = append(statuses, EvaluateViewBudget(b, in).Status)
	}
	return ComputeSweepWithRates(statuses, cfg, in.Rates, goalAllowed)
}
