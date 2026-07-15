// SPDX-License-Identifier: MIT

package budgeting

import (
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
	return p.Total.Amount > 0 && len(p.Lines) > 0
}

// BudgetCount is the number of budgets contributing to the sweep.
func (p SweepPlan) BudgetCount() int {
	return len(p.Lines)
}

// ComputeSweep totals the unspent leftover across the config's participating
// budgets. statuses are the budgets' evaluated states for the closed period
// (from Evaluate/EvaluateAll); only a positive Remaining counts — an overspent or
// exactly-spent budget contributes nothing. base is the household base currency
// the total is reported in; leftovers are summed in minor units (exact for the
// single-currency common case).
//
// goalAllowed composes the XC7 earmark-integrity gate: it is asked whether the
// target goal may receive a sweep, and when it answers false the returned plan is
// marked Blocked so the UI can explain the hold rather than silently earmark into
// an already-overdrawn account. A nil goalAllowed means "no gate" (always allow).
func ComputeSweep(statuses []Status, cfg SweepConfig, base string, goalAllowed func(goalID string) bool) SweepPlan {
	plan := SweepPlan{Total: money.Zero(base), GoalID: cfg.TargetGoalID}
	if !cfg.Enabled {
		return plan
	}
	var totalMinor int64
	for _, s := range statuses {
		if !cfg.Participates(s.Budget.ID) {
			continue
		}
		if s.Remaining.Amount <= 0 {
			continue
		}
		plan.Lines = append(plan.Lines, SweepLine{
			BudgetID:   s.Budget.ID,
			BudgetName: s.Budget.Name,
			Leftover:   s.Remaining,
		})
		totalMinor += s.Remaining.Amount
	}
	plan.Total = money.New(totalMinor, base)
	if goalAllowed != nil && cfg.TargetGoalID != "" && !goalAllowed(cfg.TargetGoalID) {
		plan.Blocked = true
	}
	return plan
}
