// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// budgetRowBarValuesResult carries the explicit results for budgetRowBarValues.
type budgetRowBarValuesResult struct {
	barSpent  string
	barPct    int
	width     int
	fillClass string
	label     string
	title     string
}

// budgetRowBarValues resolves the displayed spend, percent, fill, label, and title.
func budgetRowBarValues(s budgeting.Status, props budgetRowProps, lastMonthMode bool) budgetRowBarValuesResult {
	// Bar figures + fill (this month by default; swapped to last month below).
	barSpent := fmtMoney(s.Spent)
	barPct := s.Percent
	width := s.Percent
	fillClass := "bar-fill"
	label := uistate.T("budgets.onTrack")
	switch s.State {
	case budgeting.StateNear:
		fillClass = "bar-fill near"
		label = uistate.T("budgets.nearLimit")
	case budgeting.StateOver:
		fillClass = "bar-fill over"
		label = uistate.T("budgets.overBudget")
	default:
		// Not over/near yet, but the pace projection says this budget is trending
		// to overspend — don't claim "On track" while also warning of an overspend
		// (the L35 contradiction). Call it "At risk" instead.
		if props.PaceOver != "" {
			fillClass = "bar-fill near"
			label = uistate.T("budgets.atRisk")
		}
		// C344: three days into a period nothing has been broken yet, so "On track"
		// is a statement about the calendar, not about the household — and a card
		// that says "doing great" every time a month opens is one people learn to
		// ignore. Say what is actually true (the period just started) and let last
		// period's outcome, shown below, be the thing worth reading.
		if props.PeriodEarly {
			fillClass = "bar-fill is-hist"
			label = uistate.T("budgets.periodJustStarted")
		}
	}
	if s.PeriodStatus == budgeting.PeriodPlanned {
		anchor := props.Anchor
		if anchor.IsZero() {
			anchor = time.Now()
		}
		pr := uistate.LoadPrefs()
		start, _ := budgeting.PeriodRangeAnchored(s.Budget.Period, anchor, pr.WeekStartWeekday(), budgetPayCycleAnchor(pr.PayCycleAnchor))
		label = uistate.T("bud2.startsDate", budgetMonthName(start, false), start.Day())
	}
	if lastMonthMode {
		barSpent, barPct, width = props.LastMonthSpent, props.LastMonthPct, props.LastMonthFill
		if props.LastMonthOver {
			// An actual overrun stays danger red — it's a fact, whenever it happened.
			fillClass = "bar-fill over"
		} else {
			// History fills NEUTRAL: the healthy-green live tone (and the amber "near"
			// warning) under a LAST MONTH tag would read as a statement about now
			// (design critique: green was doing too many jobs).
			fillClass = "bar-fill is-hist"
		}
	}
	if width > 100 {
		width = 100
	}
	// Grade the over-budget treatment by HOW FAR past the cap the row is, so a small
	// overspend doesn't wear the same full-severity red as a large overrun (a mild bar
	// gets a softer tone + thinner overflow marker; a severe one, the heaviest). The
	// tier reads off the displayed percent (this-month, or last-month's in that overlay)
	// and only attaches when the fill is actually over — the style layer
	// (rules_budgetovercalm.go) does the rest.
	if strings.Contains(fillClass, "over") {
		if tc := budgeting.ClassifyOverage(barPct).Class(); tc != "" {
			fillClass += " " + tc
		}
	}

	// Show "name · category" only when they add information (see budgetTitle).
	title := budgetTitle(s.Budget.Name, props.Category)
	return budgetRowBarValuesResult{
		barSpent: barSpent, barPct: barPct, width: width, fillClass: fillClass, label: label, title: title,
	}
}
