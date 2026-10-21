// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalhistory"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// contribHistoryMonths is how far back the planned-vs-actual panel looks. A
// year is long enough to show a pattern and short enough that every bar stays
// readable at card width.
const contribHistoryMonths = 12

// goalContribHistoryNode renders a goal's contribution history as twelve months
// of planned-vs-actual (C399).
//
// The card already answers "am I on track" with a projected date, and that date
// is computed from the monthly PLAN. Nothing on the surface ever checked the
// plan against reality, so a goal funded at half its plan for six months still
// showed a confident date. This is the check: one bar per month, the plan drawn
// as a reference line across it, and a sentence naming how many months came in
// short.
//
// It renders nothing for a non-financial goal, a goal with no target, or a goal
// that has never received a contribution — an empty twelve-bar chart is a box
// that says "no data" in a very expensive way.
func goalContribHistoryNode(g domain.Goal, assignment money.Money, hasAssignment bool, assignmentErr error, now time.Time) ui.Node {
	if !g.EffectiveKind().IsFinancial() || g.TargetAmount.Amount <= 0 {
		return Fragment()
	}
	if len(g.Contributions) == 0 && len(g.ContributionHistory) == 0 {
		return Fragment()
	}
	if assignmentErr != nil {
		return goalContribHistoryUnavailable(g.ID)
	}
	cur := g.TargetAmount.Currency
	var plannedMinor int64
	if hasAssignment {
		if assignment.Currency != cur {
			return goalContribHistoryUnavailable(g.ID)
		}
		if assignment.Amount > 0 {
			plannedMinor = assignment.Amount
		}
	}
	pts, err := goalhistory.SeriesChecked(g, plannedMinor, contribHistoryMonths, now)
	if err != nil {
		return goalContribHistoryUnavailable(g.ID)
	}
	peak := goalhistory.PeakMinor(pts)
	if peak <= 0 {
		return Fragment()
	}
	sum, err := goalhistory.SummarizeChecked(pts)
	if err != nil {
		return goalContribHistoryUnavailable(g.ID)
	}

	bars := make([]any, 0, len(pts))
	for _, p := range pts {
		bar, err := contribHistoryBar(p, peak, cur)
		if err != nil {
			return goalContribHistoryUnavailable(g.ID)
		}
		bars = append(bars, bar)
	}

	// One sentence over the bars, because a reader should not have to add up
	// twelve of them. It names the shortfall as a total of monthly misses, not as
	// planned-minus-actual — a single generous month does not undo five thin ones.
	var read string
	switch {
	case plannedMinor <= 0:
		read = uistate.T("goalhist.readNoPlan", fmtMoney(money.New(sum.ActualMinor, cur)))
	case sum.OnPlan():
		read = uistate.T("goalhist.readOnPlan", sum.FundedMonths)
	default:
		read = uistate.T("goalhist.readBehind", sum.MissedMonths,
			fmtMoney(money.New(sum.ShortfallMinor, cur)))
	}

	return Div(css.Class("goal-hist"), Attr("data-testid", "goal-hist-"+g.ID),
		Div(css.Class("goal-hist-head"),
			Span(css.Class("t-caption"), uistate.T("goalhist.title")),
			Span(css.Class("muted"), Attr("data-testid", "goal-hist-read-"+g.ID), read),
		),
		// A list, not a canvas: twelve labelled rows are readable by a screen
		// reader and by a person, and the shape is just as legible.
		Div(css.Class("goal-hist-bars"), Attr("role", "list"),
			Attr("aria-label", uistate.T("goalhist.title")), bars),
		If(plannedMinor > 0, P(css.Class("goal-hist-legend"),
			uistate.T("goalhist.legend", fmtMoney(money.New(plannedMinor, cur))))),
	)
}

func goalContribHistoryUnavailable(id string) ui.Node {
	return P(css.Class("budget-sub"), Attr("data-testid", "goal-hist-unavailable-"+id), Attr("role", "status"), uistate.T("calculation.unavailable"))
}

// contribHistoryBar is one month. The plan is a marker ON the bar's track rather
// than a second bar, so "did this month clear the line" is a glance rather than
// a comparison of two lengths.
func contribHistoryBar(p goalhistory.Point, peakMinor int64, currency string) (ui.Node, error) {
	pct, err := goalhistory.PercentOfPeak(p.ActualMinor, peakMinor)
	if err != nil {
		return nil, err
	}
	planPct, err := goalhistory.PercentOfPeak(p.PlannedMinor, peakMinor)
	if err != nil {
		return nil, err
	}
	cls := "goal-hist-fill"
	if p.PlannedMinor > 0 && !p.Met() {
		cls += " is-short"
	}
	// The accessible name carries the same three facts the visual does, since a
	// bar's width is not available to a screen reader.
	aria := uistate.T("goalhist.barAria", p.Label, fmtMoney(money.New(p.ActualMinor, currency)))
	if p.PlannedMinor > 0 {
		aria = uistate.T("goalhist.barAriaPlanned", p.Label,
			fmtMoney(money.New(p.ActualMinor, currency)),
			fmtMoney(money.New(p.PlannedMinor, currency)))
	}
	var marker ui.Node = Fragment()
	if planPct > 0 {
		marker = Span(css.Class("goal-hist-plan"), Attr("aria-hidden", "true"),
			Style(map[string]string{"left": strconv.Itoa(planPct) + "%"}))
	}
	return Div(css.Class("goal-hist-row"), Attr("role", "listitem"), Attr("aria-label", aria),
		Span(css.Class("goal-hist-month"), Attr("aria-hidden", "true"), p.Label),
		Div(css.Class("goal-hist-track"), Attr("aria-hidden", "true"),
			Span(css.Class(cls), Style(map[string]string{"width": strconv.Itoa(pct) + "%"})),
			marker,
		),
		Span(css.Class("goal-hist-amt"), Attr("aria-hidden", "true"),
			fmtMoney(money.New(p.ActualMinor, currency))),
	), nil
}
