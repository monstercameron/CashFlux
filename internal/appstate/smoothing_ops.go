// SPDX-License-Identifier: MIT

package appstate

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smoothing"
)

// smoothingGoalName is the plain-English label of the system-managed sinking fund
// that holds a smoothed bill's monthly set-aside (XC3), e.g. "Set aside for
// Insurance". It is data (a goal name the user sees on the Goals screen), not UI
// chrome, so it is composed here rather than through the i18n table.
func smoothingGoalName(label string) string {
	return "Set aside for " + label
}

// syncSmoothingGoal creates, updates, or dissolves the system-managed sinking-fund
// goal that backs a smoothed recurring (XC3). When the recurring smooths, a goal
// named "Set aside for <label>" is maintained with the bill's full amount as its
// target and its due date as the deadline (so the goal's monthly set-aside matches
// the smoothing accrual). When the flag is off, any existing managed goal is
// dissolved, releasing its earmarks. Existing balance, earmarks, and ID are
// preserved across updates.
func (a *App) syncSmoothingGoal(r domain.Recurring) error {
	existing, has := smoothing.SmoothingGoalFor(a.Goals(), r.ID)

	if !r.Smooths() {
		if has {
			return a.DeleteGoal(existing.ID)
		}
		return nil
	}

	cur := r.Amount.Currency
	target := money.New(abs64(r.Amount.Amount), cur)

	g := existing
	if !has {
		g = domain.Goal{
			ID:            id.New(),
			Scope:         domain.ScopeShared,
			OwnerID:       domain.GroupOwnerID,
			CurrentAmount: money.Zero(cur),
			Custom:        map[string]any{smoothing.GoalCustomKey: r.ID},
		}
	}
	if g.Custom == nil {
		g.Custom = map[string]any{}
	}
	g.Custom[smoothing.GoalCustomKey] = r.ID
	g.Name = smoothingGoalName(r.Label)
	g.IsSinkingFund = true
	g.CategoryID = r.CategoryID
	g.TargetAmount = target
	g.TargetDate = r.NextDue
	if g.CurrentAmount.Currency == "" {
		g.CurrentAmount = money.Zero(cur)
	}
	return a.PutGoal(g)
}

// dissolveSmoothingGoal removes the system-managed sinking-fund goal owned by
// recurringID, if one exists — used when the recurring is deleted so the fund and
// its earmarks are released.
func (a *App) dissolveSmoothingGoal(recurringID string) error {
	if g, has := smoothing.SmoothingGoalFor(a.Goals(), recurringID); has {
		return a.DeleteGoal(g.ID)
	}
	return nil
}

// abs64 returns the absolute value of a signed minor-unit amount.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
