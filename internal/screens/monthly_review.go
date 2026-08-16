// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — monthly_review.go is the guided month-end review (AG10).
//
// It is a card on the Insights tab, not a modal and not a route. That is the whole
// difference between a ritual people finish and one they close: a modal demands the
// ten minutes now, and the answer to a demand made at the wrong moment is always
// "no". A card waits, remembers where it got to, and can be walked away from
// mid-step without losing anything.
//
// Every step ends in an action or an explicit skip. Skipping is a real outcome
// here — the goal is to reach the end having DECIDED about each step, and "not this
// month" is a decision — so the close says what was left rather than implying
// everything was handled.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/goalcoach"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/monthlyreview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// MonthlyReviewCard renders the review, or nothing when there is nothing to
// review, when it has already been done this month, or when it was dismissed.
func MonthlyReviewCard() ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	_ = uistate.UseReviewRevision().Get()
	nav := router.UseNavigate()

	next := ui.UseEvent(Prevent(func() {
		p, a := uistate.LoadReviewProgress(), reviewAvailability(appstate.Default)
		uistate.SaveReviewProgress(monthlyreview.Advance(p, a, time.Now(), false))
	}))
	skip := ui.UseEvent(Prevent(func() {
		p, a := uistate.LoadReviewProgress(), reviewAvailability(appstate.Default)
		uistate.SaveReviewProgress(monthlyreview.Advance(p, a, time.Now(), true))
	}))
	dismiss := ui.UseEvent(Prevent(func() {
		uistate.SaveReviewProgress(monthlyreview.Dismiss(uistate.LoadReviewProgress(), time.Now()))
	}))
	openStep := ui.UseEvent(Prevent(func() {
		st := monthlyreview.Resolve(uistate.LoadReviewProgress(), reviewAvailability(appstate.Default), time.Now())
		nav.Navigate(uistate.RoutePath(reviewStepRoute(st.Current)))
	}))

	if app == nil {
		return Fragment()
	}
	progress := uistate.LoadReviewProgress()
	avail := reviewAvailability(app)
	if !monthlyreview.ShouldOffer(progress, avail, time.Now()) {
		return Fragment()
	}
	state := monthlyreview.Resolve(progress, avail, time.Now())

	body := reviewStepBody(app, state.Current, progress, avail)
	nextLabel := uistate.T("review.next")
	if state.AtLast() {
		nextLabel = uistate.T("review.finish")
	}

	return Div(css.Class("mrev"), Attr("data-testid", "monthly-review"),
		Attr("role", "region"), Attr("aria-label", uistate.T("review.title")),
		Div(css.Class("mrev-head"),
			Div(
				P(css.Class("mrev-eyebrow"), uistate.T("review.eyebrow",
					reviewStepNumber(state), len(state.Steps))),
				H2(css.Class("mrev-title"), uistate.T(reviewStepTitleKey(state.Current))),
			),
			Button(css.Class("mrev-close"), Type("button"), Attr("data-testid", "monthly-review-dismiss"),
				Attr("aria-label", uistate.T("review.dismiss")), Title(uistate.T("review.dismiss")),
				OnClick(dismiss), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		Div(css.Class("mrev-body"), Attr("data-testid", "monthly-review-body"), body),
		Div(css.Class("mrev-actions"),
			// The action comes first and the skip second, but the skip is a plain
			// button rather than a hidden link: a ritual that makes leaving feel
			// furtive is one people leave for good.
			If(state.Current != monthlyreview.StepClose && state.Current != monthlyreview.StepRecap,
				Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "monthly-review-open"),
					OnClick(openStep), uistate.T(reviewStepActionKey(state.Current)))),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "monthly-review-next"),
				OnClick(next), nextLabel),
			If(state.Current != monthlyreview.StepClose && state.Current != monthlyreview.StepRecap,
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"), Attr("data-testid", "monthly-review-skip"),
					OnClick(skip), uistate.T("review.skip"))),
		),
	)
}

// reviewStepNumber is the 1-based position for the "step 2 of 4" line.
func reviewStepNumber(s monthlyreview.State) int { return s.Index + 1 }

// reviewAvailability works out which steps have anything in them this month.
func reviewAvailability(app *appstate.App) monthlyreview.Availability {
	if app == nil {
		return monthlyreview.Availability{}
	}
	var a monthlyreview.Availability
	a.HasFindings = len(runAnomalyDetectors(app, uistate.LoadPrefs().WeekStartWeekday())) > 0

	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: settings.FXRates}
	mStart, mEnd := dateutil.MonthRange(time.Now())
	if statuses, err := budgeting.EvaluateAll(app.Budgets(), app.Transactions(), mStart, mEnd, rates, budgeting.DefaultNearThreshold); err == nil {
		for _, st := range statuses {
			if st.State == budgeting.StateOver || st.State == budgeting.StateNear {
				a.BudgetsNeedingAttention++
			}
		}
	}
	goals := make([]goalcoach.Goal, 0, len(app.Goals()))
	for _, g := range app.Goals() {
		goals = append(goals, coachGoal(g))
	}
	a.GoalsNeedingAttention = len(goalcoach.CoachAll(goals, time.Now()))
	return a
}

// reviewStepBody renders what the current step has to say. Each is a couple of
// lines: the card is the doorway to the work, not the work itself, and a step that
// tries to be both ends up too big to read and too small to act in.
func reviewStepBody(app *appstate.App, step monthlyreview.Step, p monthlyreview.Progress, a monthlyreview.Availability) ui.Node {
	switch step {
	case monthlyreview.StepFindings:
		return P(uistate.T("review.findingsBody", len(runAnomalyDetectors(app, uistate.LoadPrefs().WeekStartWeekday()))))
	case monthlyreview.StepBudgets:
		return P(uistate.T("review.budgetsBody", a.BudgetsNeedingAttention))
	case monthlyreview.StepGoals:
		return P(uistate.T("review.goalsBody", a.GoalsNeedingAttention))
	case monthlyreview.StepClose:
		return Fragment(
			P(monthlyreview.CloseSummary(p, a, time.Now())),
			P(css.Class("mrev-fine"), uistate.T("review.closeFine")),
		)
	default:
		return P(uistate.T("review.recapBody"))
	}
}

// reviewStepRoute is where a step's work actually happens. The card never tries to
// host the work: the budgets screen is better at budgets than a card ever will be,
// and sending somebody there means they finish in the place they already know.
func reviewStepRoute(step monthlyreview.Step) string {
	switch step {
	case monthlyreview.StepFindings:
		return "/assistant"
	case monthlyreview.StepBudgets:
		return "/budgets"
	case monthlyreview.StepGoals:
		return "/goals"
	default:
		return "/insights"
	}
}

// reviewStepTitleKey and reviewStepActionKey name a step and its button.
func reviewStepTitleKey(step monthlyreview.Step) string {
	switch step {
	case monthlyreview.StepFindings:
		return "review.findingsTitle"
	case monthlyreview.StepBudgets:
		return "review.budgetsTitle"
	case monthlyreview.StepGoals:
		return "review.goalsTitle"
	case monthlyreview.StepClose:
		return "review.closeTitle"
	default:
		return "review.recapTitle"
	}
}

func reviewStepActionKey(step monthlyreview.Step) string {
	switch step {
	case monthlyreview.StepFindings:
		return "review.findingsAction"
	case monthlyreview.StepBudgets:
		return "review.budgetsAction"
	case monthlyreview.StepGoals:
		return "review.goalsAction"
	default:
		return "review.recapAction"
	}
}
