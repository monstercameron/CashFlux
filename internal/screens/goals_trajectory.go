// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goaltrajectory"
	"github.com/monstercameron/CashFlux/internal/money"
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

	monthly, ok, _ := goalsvc.MonthlyAssignment(g, now)
	hasContribution := ok && monthly.Amount > 0

	heading := Div(css.Class("gtj-head"), uistate.T("goaltrajectory.heading"))

	// Coverage-aware: the projection starts from everything already accounted
	// for — saved PLUS earmarked — so setting money aside moves the trajectory
	// the moment it happens, month to month, exactly like the card's figures.
	covered := goalsvc.CoverageMinor(g)

	// A goal that's already covered has no pace left to project — render nothing.
	// The card already announces completion once (the 100% loader + the "Funded —
	// reallocate?" line); a trajectory sentence would restate it, and the old
	// behaviour (falling through to the "add a monthly contribution" prompt) told
	// a funded goal to keep planning. Checked BEFORE the no-contribution prompt.
	if covered >= g.TargetAmount.Amount {
		return Fragment()
	}

	// No pace to project from: a low-pressure prompt, no chart.
	if !hasContribution {
		return Div(css.Class("gtj"), Attr("data-testid", "goal-trajectory-"+g.ID),
			heading,
			Div(css.Class("gtj-eta is-muted"), Attr("data-testid", "goal-trajectory-eta-"+g.ID),
				uistate.T("goaltrajectory.empty")),
		)
	}
	res := goaltrajectory.Project(goaltrajectory.Input{
		CurrentMinor: covered,
		TargetMinor:  g.TargetAmount.Amount,
		MonthlyMinor: monthly.Amount,
		Start:        now,
		TargetDate:   g.TargetDate,
	})

	targetStr := fmtMoney(g.TargetAmount)
	nowStr := fmtMoney(money.New(covered, g.TargetAmount.Currency))
	hasTargetDate := !g.TargetDate.IsZero()
	monthsToGoal := res.MonthsToGoal

	// The plain-language ETA sentence (wording unchanged) — the precise, screen-reader
	// friendly summary beneath the rail. (A covered goal already returned above.)
	var readout ui.Node
	switch {
	case !res.Reachable:
		readout = Span(uistate.T("goaltrajectory.beyond", targetStr))
	case hasTargetDate:
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
		if monthsToGoal == 1 {
			readout = Span(uistate.T("goaltrajectory.onTrackOneMonth", targetStr,
				res.ProjectedDate.Format("January 2006")))
		} else {
			readout = Span(uistate.T("goaltrajectory.onTrack", targetStr,
				res.ProjectedDate.Format("January 2006"), monthsToGoal))
		}
	}

	// A fixed monthly contribution grows the balance LINEARLY, so a chart is just a big
	// diagonal triangle — large and dull. Show a compact "now → horizon" pace rail
	// instead: a fill up to the month the goal is reached, a flag at that point, and,
	// when a deadline exists, the target-date tick — so ahead/behind reads spatially
	// (flag left of the tick = ahead) in ~a third of the height.
	clamp01 := func(f float64) float64 {
		if f < 0 {
			return 0
		}
		if f > 1 {
			return 1
		}
		return f
	}
	// railPos keeps a marker inside a safe inner band so it never clips at the edges.
	railPos := func(f float64) string { return fmt.Sprintf("%.1f%%", 4+clamp01(f)*92) }

	tone, pillTone, pillText := "is-ahead", "is-ahead", ""
	fillFrac := 1.0
	flagFrac, targFrac := -1.0, -1.0 // <0 => marker not drawn
	horizonMonth := res.ProjectedDate
	onPace := false // exactly on pace for the target date → the rail adds nothing

	switch {
	case !res.Reachable:
		// No landing point at this pace: show current progress as an amber sliver and the
		// deadline tick, so the shortfall is visible without a fake projection.
		tone, pillTone, pillText = "is-behind", "is-behind", uistate.T("goaltrajectory.pillOffPace")
		fillFrac = clamp01(float64(covered) / float64(g.TargetAmount.Amount))
		if hasTargetDate {
			targFrac, horizonMonth = 1, g.TargetDate
		}
	case hasTargetDate:
		monthsToTarget := monthsBetween(now, g.TargetDate)
		if monthsToTarget < 0 {
			monthsToTarget = 0
		}
		span := monthsToGoal
		if monthsToTarget > span {
			span = monthsToTarget
		}
		if span < 1 {
			span = 1
		}
		flagFrac = clamp01(float64(monthsToGoal) / float64(span))
		targFrac = clamp01(float64(monthsToTarget) / float64(span))
		fillFrac = flagFrac
		slack := monthsBetween(res.ProjectedDate, g.TargetDate)
		if monthsToGoal <= monthsToTarget {
			tone, pillTone = "is-ahead", "is-ahead"
			switch {
			case slack <= 0:
				pillText = uistate.T("goaltrajectory.pillOnPace")
				onPace = true
			case slack == 1:
				pillText = uistate.T("goaltrajectory.pillAheadOne")
			default:
				pillText = uistate.T("goaltrajectory.pillAhead", slack)
			}
		} else {
			tone, pillTone = "is-behind", "is-behind"
			if slack == -1 {
				pillText = uistate.T("goaltrajectory.pillBehindOne")
			} else {
				pillText = uistate.T("goaltrajectory.pillBehind", -slack)
			}
		}
		if res.ProjectedDate.After(g.TargetDate) {
			horizonMonth = res.ProjectedDate
		} else {
			horizonMonth = g.TargetDate
		}
	default: // reachable, no deadline — the runway simply lands on the projected month
		tone, pillTone = "is-ahead", "is-neutral"
		pillText = uistate.T("goaltrajectory.pillHits", res.ProjectedDate.Format("Jan 2006"))
		fillFrac, horizonMonth = 1, res.ProjectedDate
	}

	// Compact mode: when the rail's spatial read carries no information — there's no
	// target date to diverge from, or the projection lands exactly on pace — the
	// one-line readout says everything, so the pill + rail + end captions stay home
	// and the card gives that height back.
	if !hasTargetDate || onPace {
		return Div(css.Class("gtj"), Attr("data-testid", "goal-trajectory-"+g.ID),
			heading,
			Div(css.Class("gtj-eta"), Attr("data-testid", "goal-trajectory-eta-"+g.ID), readout),
		)
	}

	railKids := []any{css.Class("gtj-rail")}
	railKids = append(railKids, Div(ClassStr("gtj-rail-fill "+tone),
		Attr("style", fmt.Sprintf("width:%.1f%%", clamp01(fillFrac)*100))))
	if targFrac >= 0 {
		railKids = append(railKids, Div(css.Class("gtj-rail-target"), Attr("style", "left:"+railPos(targFrac)),
			Attr("aria-hidden", "true"), Title(uistate.T("goaltrajectory.legendTarget"))))
	}
	if flagFrac >= 0 {
		railKids = append(railKids, Div(ClassStr("gtj-rail-flag "+tone), Attr("style", "left:"+railPos(flagFrac)),
			Attr("aria-hidden", "true"), Title(uistate.T("goaltrajectory.legendHits"))))
	}

	horizonStr := horizonMonth.Format("Jan '06")

	// The readout sentence renders under the rail ONLY when it adds something the
	// pill can't say — the behind / off-pace advice ("consider a larger monthly
	// amount"). An ahead sentence would restate the pill + rail verbatim, which is
	// the pace-said-twice noise this card was just cured of.
	var etaNode ui.Node = Fragment()
	if tone == "is-behind" {
		etaNode = Div(css.Class("gtj-eta"), Attr("data-testid", "goal-trajectory-eta-"+g.ID), readout)
	}

	return Div(css.Class("gtj"), Attr("data-testid", "goal-trajectory-"+g.ID),
		Attr("aria-label", uistate.T("goaltrajectory.railAria", nowStr, targetStr, pillText)),
		Div(css.Class("gtj-head2"),
			Span(css.Class("gtj-head"), uistate.T("goaltrajectory.heading")),
			Span(ClassStr("gtj-pill "+pillTone), pillText),
		),
		Div(railKids...),
		Div(css.Class("gtj-rail-ends"), Attr("aria-hidden", "true"),
			Span(uistate.T("goaltrajectory.railNow", nowStr)),
			Span(uistate.T("goaltrajectory.railTarget", targetStr, horizonStr)),
		),
		etaNode,
	)
}

// monthsBetween returns the whole-month difference from `from` to `to` (positive
// when `to` is later). Used to phrase the trajectory as slack against a target date.
func monthsBetween(from, to time.Time) int {
	return (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
}
