// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goaltrajectory"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// goalTrajectoryNode renders a goal card's savings-trajectory section: a small
// heading, a reused AreaChart of the projected balance over time, and a one-line
// ETA readout. It is a projection of the goal's own pace — the monthly assignment
// from goalsvc.MonthlyAssignment (explicit contribution, else the target-date
// pace) accrued from the current balance toward the target.
//
// It renders a compact empty-state line (no chart) when there is no contribution
// to project from, and nothing at all for a non-financial goal or one without a
// positive target — so a card never shows an empty box. `now` is the reference
// date the projection starts from (passed in for testability / a single clock).
func goalTrajectoryNode(g domain.Goal, now time.Time) ui.Node {
	// Only financial goals with a real target have a money trajectory.
	if !g.EffectiveKind().IsFinancial() || g.TargetAmount.Amount <= 0 {
		return Fragment()
	}

	cur := g.TargetAmount.Currency
	monthly, ok, _ := goalsvc.MonthlyAssignment(g, now)
	hasContribution := ok && monthly.Amount > 0

	heading := Div(css.Class("gtj-head"), uistate.T("goaltrajectory.heading"))

	// No pace to project from: a low-pressure prompt, no chart.
	if !hasContribution {
		return Div(css.Class("gtj"), Attr("data-testid", "goal-trajectory-"+g.ID),
			heading,
			Div(css.Class("gtj-eta is-muted"), Attr("data-testid", "goal-trajectory-eta-"+g.ID),
				uistate.T("goaltrajectory.empty")),
		)
	}

	res := goaltrajectory.Project(goaltrajectory.Input{
		CurrentMinor: g.CurrentAmount.Amount,
		TargetMinor:  g.TargetAmount.Amount,
		MonthlyMinor: monthly.Amount,
		Start:        now,
		TargetDate:   g.TargetDate,
	})

	// Convert the int64 minor-unit series into the AreaChart's float64 Values, with
	// per-point money ValueLabels for hover and sparse month captions for the axis.
	n := len(res.Series)
	values := make([]float64, n)
	valueLabels := make([]string, n)
	labels := make([]string, n)
	labelStep := (n - 1) / 4
	if labelStep < 1 {
		labelStep = 1
	}
	for i, p := range res.Series {
		values[i] = float64(p.BalanceMinor)
		valueLabels[i] = fmtMoney(money.New(p.BalanceMinor, cur))
		// Sparse captions: first, last, and every labelStep in between — the rest are
		// empty spans so the axis row stays uncluttered on long projections.
		if i == 0 || i == n-1 || i%labelStep == 0 {
			labels[i] = p.Month.Format("Jan '06")
		}
	}

	targetStr := fmtMoney(g.TargetAmount)
	hasTargetDate := !g.TargetDate.IsZero()
	var readout ui.Node
	switch {
	case res.Reachable && res.MonthsToGoal == 0:
		readout = Span(uistate.T("goaltrajectory.reachedNow", targetStr))
	case !res.Reachable:
		readout = Span(uistate.T("goaltrajectory.beyond", targetStr))
	case hasTargetDate:
		// A target DATE is already shown in the card's stat row, so foreground the
		// slack against it rather than restating the date. Positive slack = the pace
		// lands ahead of the target; negative = behind.
		tMonth := g.TargetDate.Format("Jan 2006")
		slack := monthsBetween(res.ProjectedDate, g.TargetDate)
		switch {
		case slack == 0:
			readout = Span(uistate.T("goaltrajectory.onPace", tMonth))
		case slack == 1:
			readout = Span(uistate.T("goaltrajectory.aheadOne", tMonth))
		case slack > 1:
			readout = Span(uistate.T("goaltrajectory.ahead", slack, tMonth))
		case slack == -1:
			readout = Span(uistate.T("goaltrajectory.behindOne", tMonth))
		default:
			readout = Span(uistate.T("goaltrajectory.behind", -slack, tMonth))
		}
	default:
		// Reachable, no target date set — the projection supplies the landing month.
		if res.MonthsToGoal == 1 {
			readout = Span(uistate.T("goaltrajectory.onTrackOneMonth", targetStr,
				res.ProjectedDate.Format("January 2006")))
		} else {
			readout = Span(uistate.T("goaltrajectory.onTrack", targetStr,
				res.ProjectedDate.Format("January 2006"), res.MonthsToGoal))
		}
	}

	return Div(css.Class("gtj"), Attr("data-testid", "goal-trajectory-"+g.ID),
		heading,
		Div(css.Class("gtj-chart"),
			uiw.AreaChart(uiw.AreaChartProps{
				Values:      values,
				Labels:      labels,
				ValueLabels: valueLabels,
				Stroke:      uistate.CurrentAccent(),
				GradientID:  "gtj-" + g.ID,
				Label:       uistate.T("goaltrajectory.chartLabel", g.Name),
			}),
		),
		Div(css.Class("gtj-eta"), Attr("data-testid", "goal-trajectory-eta-"+g.ID), readout),
	)
}

// monthsBetween returns the whole-month difference from `from` to `to` (positive
// when `to` is later). Used to phrase the trajectory as slack against a target date.
func monthsBetween(from, to time.Time) int {
	return (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
}
