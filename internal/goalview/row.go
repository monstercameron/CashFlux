// SPDX-License-Identifier: MIT

package goalview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

// RowModel holds the shared facts behind compact and expanded goal cards.
// The original stored Goal remains separate, exclusively for edit callbacks.
type RowModel struct {
	Evaluation   goals.Evaluation
	Tasks        goals.TaskSummary
	Growth       goals.GrowthProjection
	Now          time.Time
	RetargetDate time.Time
	HasRetarget  bool
}

// Retarget derives a deadline from the same converted remaining amount and
// base-currency fair share used by the page's health verdict.
func Retarget(g domain.Goal, e goals.Evaluation, p Pace, base string, rates currency.Rates, now time.Time) (time.Time, bool) {
	if e.Err != nil || e.Complete || p.FairMinor <= 0 {
		return time.Time{}, false
	}
	fair, err := rates.Convert(money.New(p.FairMinor, base), g.TargetAmount.Currency)
	if err != nil {
		return time.Time{}, false
	}
	if g.IsPaused(now) {
		now = g.PausedUntil
	}
	return goals.WorkableDate(e.Coverage.Remaining.Amount, fair.Amount, now)
}

// BuildRow evaluates progress once and computes an optional growth projection.
// Unknown coverage cannot produce a financial projection.
func BuildRow(g domain.Goal, tasks goals.TaskSummary, rates currency.Rates, now time.Time) RowModel {
	r := RowModel{Evaluation: goals.EvaluateGoal(g, tasks, rates, now), Tasks: tasks, Now: now}
	if !g.IsFinancial() || r.Evaluation.Err != nil || g.ExpectedReturnBips <= 0 || r.Evaluation.Complete {
		return r
	}
	monthly := r.Evaluation.MonthlyNeeded
	if !r.Evaluation.HasMonthlyNeeded {
		var err error
		if g.MonthlyContribution.Amount == 0 {
			monthly = money.Zero(g.TargetAmount.Currency)
		} else {
			monthly, err = rates.Convert(g.MonthlyContribution, g.TargetAmount.Currency)
		}
		if err != nil {
			return r
		}
	}
	r.Growth = goals.ProjectWithGrowth(r.Evaluation.Coverage.Covered.Amount, g.TargetAmount.Amount, monthly.Amount, g.ExpectedReturnBips, now)
	return r
}
