// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// GoalPauseFormProps drives the pause-a-goal form rendered in the shell-root flip
// modal (GoalEditHost, mode "pause").
type GoalPauseFormProps struct {
	GoalID string
	OnDone func()
}

// GoalPauseForm lets the user pause a goal for a number of months and shows the
// HONEST cost of doing so BEFORE confirming ("pausing 2 months moves the finish
// from January to March") — GL7. Pausing is framed as a deliberate choice, never
// a failure: the copy is low-pressure and an already-paused goal can be resumed
// from here. It owns its state and persists through appstate.PauseGoal.
func GoalPauseForm(props GoalPauseFormProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	pr := uistate.UsePrefs().Get()
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	var g domain.Goal
	found := false
	if app != nil {
		for _, gg := range app.Goals() {
			if gg.ID == props.GoalID {
				g, found = gg, true
				break
			}
		}
	}

	monthsS := ui.UseState("2")
	cancel := ui.UseEvent(Prevent(func() { done() }))
	resume := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		if _, err := app.ResumeGoal(props.GoalID); err == nil {
			uistate.PostNotice(uistate.T("goals.resumedToast"), false)
			uistate.BumpDataRevision()
		}
		done()
	}))
	doPause := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		months, _ := strconv.Atoi(monthsS.Get())
		if months < 1 {
			months = 1
		}
		res, err := app.PauseGoal(props.GoalID, months, time.Now())
		if err != nil {
			return
		}
		uistate.PostNotice(uistate.T("goals.pausedToast", pr.FormatDate(res.PausedUntil)), false)
		uistate.BumpDataRevision()
		done()
	}))

	if app == nil || !found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	now := time.Now()
	months, _ := strconv.Atoi(monthsS.Get())
	if months < 1 {
		months = 1
	}

	// The effective monthly used to project the finish (explicit contribution or
	// target-date pace); zero when the goal has neither.
	monthly := money.Zero(g.TargetAmount.Currency)
	if m, ok, _ := goalsvc.MonthlyAssignment(g, now); ok {
		monthly = m
	}

	// Honest cost preview: where the finish moves if we pause this long.
	var costLine ui.Node
	if cost, err := goalsvc.ComputePauseCost(g, monthly, now, months); err == nil && cost.HasFinish {
		if months == 1 {
			costLine = P(css.Class("t-caption"), Attr("role", "status"), Attr("data-testid", "goal-pause-cost"),
				uistate.T("goals.pauseCostOne", pr.FormatDate(cost.Original), pr.FormatDate(cost.Shifted)))
		} else {
			costLine = P(css.Class("t-caption"), Attr("role", "status"), Attr("data-testid", "goal-pause-cost"),
				uistate.T("goals.pauseCost", months, pr.FormatDate(cost.Original), pr.FormatDate(cost.Shifted)))
		}
	} else {
		costLine = P(css.Class("t-caption", "muted"), Attr("role", "status"), Attr("data-testid", "goal-pause-cost"),
			uistate.T("goals.pauseCostNoFinish"))
	}

	// Month picker: 1..12.
	opts := make([]uiw.SelectOption, 0, 12)
	for i := 1; i <= 12; i++ {
		label := uistate.T("goals.pauseMonths", i)
		if i == 1 {
			label = uistate.T("goals.pauseOneMonth")
		}
		opts = append(opts, uiw.SelectOption{Value: strconv.Itoa(i), Label: label})
	}

	// An already-paused goal can be resumed straight from here.
	var resumeRow ui.Node = Fragment()
	if g.IsPaused(now) {
		resumeRow = P(css.Class("t-caption"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "goal-pause-resume"), OnClick(resume),
				uistate.T("goals.resumeAction")))
	}

	return Form(css.Class("acct-edit-form", "goal-pause"), OnSubmit(doPause),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", "muted"), Style(map[string]string{"margin": "0"}), uistate.T("goals.pauseIntro")),
			labeledField(uistate.T("goals.pauseForLabel"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: opts, Selected: monthsS.Get(), TestID: "goal-pause-months",
					OnChange: func(v string) { monthsS.Set(v) }, AriaLabel: uistate.T("goals.pauseForLabel"),
				})),
			costLine,
			resumeRow,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "goal-pause-confirm"), uistate.T("goals.pauseConfirm")),
		),
	)
}
