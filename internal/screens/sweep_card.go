// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// sweep_card.go is the UI shell for the leftover-sweep ritual (XC6): the quiet
// month-boundary card on /budgets and the "Sweep leftovers" config flip modal
// reachable from the budgets toolbar. All computation lives in the tested
// budgeting.ComputeSweep / appstate.ApplyLeftoverSweep; this file only renders.

// closedMonthRange returns the previous calendar month's [start, end) range and
// its "YYYY-MM" key, relative to now. Monthly periods ignore week-start.
func closedMonthRange(now time.Time) (start, end time.Time, key string) {
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthRef := thisMonthStart.Add(-time.Nanosecond)
	start, end = budgeting.PeriodRange(domain.PeriodMonthly, lastMonthRef, time.Sunday)
	return start, end, lastMonthRef.Format("2006-01")
}

// goalNameByID looks up a goal's display name, falling back to "your goal".
func goalNameByID(app *appstate.App, id string) string {
	for _, g := range app.Goals() {
		if g.ID == id {
			return g.Name
		}
	}
	return uistate.T("sweep.fallbackGoal")
}

// budgetCountPhrase renders "1 budget" / "N budgets".
func budgetCountPhrase(n int) string {
	if n == 1 {
		return uistate.T("sweep.budgetsOne")
	}
	return uistate.T("sweep.budgetsMany", n)
}

// budgetsSweepCard renders the dismissible month-close sweep card, or Fragment()
// when there is nothing to sweep, the ritual is off, or this month was already
// handled. One sentence, one primary action ("Sweep $X to Goal"), one dismiss —
// a quiet month-boundary moment, never naggy. Its own component so the dismissal
// hook stays at a stable render position.
func budgetsSweepCard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	cfg := uistate.SweepConfigGet()
	now := time.Now()
	_, _, monthKey := closedMonthRange(now)

	// Local hide-state so a click hides the card instantly (the handled-month is
	// also persisted so it never returns for this month).
	hidden := ui.UseState(false)

	if !uistate.SweepPromptDue(monthKey, cfg) || hidden.Get() {
		return Fragment()
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	mStart, mEnd, _ := closedMonthRange(now)
	statuses, err := budgeting.EvaluateAll(app.Budgets(), app.Transactions(), mStart, mEnd, rates, budgeting.DefaultNearThreshold)
	if err != nil {
		return Fragment()
	}
	plan := budgeting.ComputeSweep(statuses, cfg.Domain(), base, app.SweepAllowedForGoal)
	if !plan.HasLeftover() {
		return Fragment()
	}

	goalName := goalNameByID(app, cfg.TargetGoalID)
	dec := currency.Decimals(base)
	totalStr := money.FormatMinor(plan.Total.Amount, dec)

	onDismiss := ui.UseEvent(func() {
		uistate.MarkSweepPromptHandled(monthKey)
		uistate.RequestPersist()
		hidden.Set(true)
	})
	onSweep := ui.UseEvent(func() {
		if _, err := app.ApplyLeftoverSweep(cfg.TargetGoalID, plan.Total.Amount, base); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.MarkSweepPromptHandled(monthKey)
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("sweep.done", totalStr, goalName), false)
		hidden.Set(true)
	})
	nav := router.UseNavigate()
	onReview := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/goals")) })

	// The primary action, or — when the target goal's account is over-earmarked
	// (XC7 gate) — a quiet explanation with a "Review goals" nudge instead.
	var action ui.Node
	if plan.Blocked {
		action = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mt3),
			P(css.Class("t-caption", tw.TextDim), uistate.T("sweep.blocked", goalName)),
			Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "sweep-review-goals"), OnClick(onReview),
				uistate.T("integrity.reviewGoals")),
		)
	} else {
		action = Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "sweep-approve"),
				OnClick(onSweep),
				uistate.T("sweep.sweepAction", totalStr, goalName)),
			Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
				Attr("data-testid", "sweep-dismiss"), OnClick(onDismiss),
				uistate.T("sweep.dismiss")),
		)
	}

	return Div(
		css.Class("catchup-card"),
		Attr("role", "complementary"),
		Attr("data-testid", "sweep-card"),
		Attr("aria-label", uistate.T("sweep.cardTitle")),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), "🧹"),
				Div(css.Class("catchup-card-text"),
					Strong(uistate.T("sweep.cardTitle")),
					P(uistate.T("sweep.cardBody", totalStr, budgetCountPhrase(plan.BudgetCount()), goalName)),
				),
			),
			action,
		),
	)
}

// sweepConfigToolbarButton is the "Sweep leftovers" btn-tool for the budgets
// toolbar. It opens the config flip modal (rendered from the surface root). Its
// own component so its click hook stays at a stable position.
func sweepConfigToolbarButton() ui.Node {
	openAtom := uistate.UseSweepConfigOpen()
	onOpen := ui.UseEvent(Prevent(func() { openAtom.Set(true) }))
	return Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-sweep-config"), Title(uistate.T("sweep.openConfig")),
		OnClick(onOpen),
		uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("sweep.openConfig")))
}

// budgetsSweepConfigModal renders the "Sweep leftovers" flip modal when its open
// atom is set, or Fragment() otherwise. It is rendered as a sibling of the bento
// (not inside a tile) so no tile transform breaks its centering.
func budgetsSweepConfigModal() ui.Node {
	openAtom := uistate.UseSweepConfigOpen()
	if !openAtom.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("sweep.configTitle"),
		Width:    uiw.FlipMediumW,
		Height:   uiw.FlipMediumH,
		NoFooter: true,
		OnClose:  func() { openAtom.Set(false) },
		Back:     ui.CreateElement(sweepConfigForm, sweepConfigFormProps{OnDone: func() { openAtom.Set(false) }}),
	})
}

type sweepConfigFormProps struct {
	OnDone func()
}

// sweepConfigForm is the staged Save/Cancel config body: enable toggle, which
// budgets participate, and the destination goal. Edits are held in local draft
// state and only committed on Save (staged, like other configs).
func sweepConfigForm(props sweepConfigFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()
	cfg := uistate.SweepConfigGet()

	enabled := ui.UseState(cfg.Enabled)
	goalID := ui.UseState(cfg.TargetGoalID)
	// Selected budgets held as a set; seed from the saved config.
	seed := map[string]bool{}
	for _, id := range cfg.BudgetIDs {
		seed[id] = true
	}
	selected := ui.UseState(seed)

	onToggleEnabled := ui.UseEvent(func() { enabled.Set(!enabled.Get()) })

	budgets := app.Budgets()
	goals := app.Goals()

	onSave := ui.UseEvent(Prevent(func() {
		ids := make([]string, 0, len(selected.Get()))
		for _, b := range budgets { // stable, store order
			if selected.Get()[b.ID] {
				ids = append(ids, b.ID)
			}
		}
		uistate.SetSweepConfig(uistate.SweepConfig{
			Enabled:      enabled.Get(),
			BudgetIDs:    ids,
			TargetGoalID: goalID.Get(),
		})
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))
	onCancel := ui.UseEvent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	})

	// Budget checkbox rows — each its own component (no On* in a loop). They sit in a
	// bordered inset list so the participation choices read as one aligned group.
	var budgetRows ui.Node
	if len(budgets) == 0 {
		budgetRows = P(css.Class("t-caption", tw.TextDim), uistate.T("sweep.configNoBudgets"))
	} else {
		rows := make([]ui.Node, 0, len(budgets))
		for _, b := range budgets {
			bid := b.ID
			rows = append(rows, ui.CreateElement(sweepBudgetCheckRow, sweepBudgetRowProps{
				ID: bid, Name: b.Name, Checked: selected.Get()[bid],
				OnToggle: func() {
					next := map[string]bool{}
					for k, v := range selected.Get() {
						next[k] = v
					}
					next[bid] = !next[bid]
					selected.Set(next)
				},
			}))
		}
		budgetRows = Div(css.Class("sweep-budgets"), rows)
	}

	// Destination goal — the standard bordered .field select (via SelectInput), matching
	// every other config modal's picker.
	var goalPicker ui.Node
	if len(goals) == 0 {
		goalPicker = P(css.Class("t-caption", tw.TextDim), uistate.T("sweep.configNoGoals"))
	} else {
		opts := make([]uiw.SelectOption, 0, len(goals)+1)
		opts = append(opts, uiw.SelectOption{Value: "", Label: uistate.T("sweep.configGoalNone")})
		for _, g := range goals {
			opts = append(opts, uiw.SelectOption{Value: g.ID, Label: g.Name})
		}
		goalPicker = uiw.SelectInput(uiw.SelectInputProps{
			Options: opts, Selected: goalID.Get(),
			OnChange:  func(v string) { goalID.Set(v) },
			AriaLabel: uistate.T("sweep.configGoal"), TestID: "sweep-config-goal",
		})
	}

	// Standard config-modal shell: an .acct-edit-form with a scrolling body and a pinned
	// Save/Cancel foot, identical-in-kind to the budget edit modal.
	return Form(css.Class("acct-edit-form"), OnSubmit(onSave),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("sweep.configIntro")),
			// Enable toggle as a standard bordered toggle row (matches the budget editor's
			// rollover row).
			Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2),
				Attr("style", "flex-wrap:nowrap;cursor:pointer"),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"),
					Attr("data-testid", "sweep-config-enable"), OnChange(onToggleEnabled)},
					checkedAttr(enabled.Get())...)...),
				Span(css.Class("t-body"), uistate.T("sweep.configEnable")),
			),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
				Span(css.Class("t-caption", tw.TextDim), uistate.T("sweep.configBudgets")),
				budgetRows,
			),
			labeledField(uistate.T("sweep.configGoal"), goalPicker),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"),
				Attr("data-testid", "sweep-config-cancel"), OnClick(onCancel), uistate.T("sweep.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"),
				Attr("data-testid", "sweep-config-save"), uistate.T("sweep.save")),
		),
	)
}

type sweepBudgetRowProps struct {
	ID       string
	Name     string
	Checked  bool
	OnToggle func()
}

// sweepBudgetCheckRow is one budget's participation checkbox as its own component
// so its OnChange hook stays at a stable render position.
func sweepBudgetCheckRow(props sweepBudgetRowProps) ui.Node {
	onChange := ui.UseEvent(func() {
		if props.OnToggle != nil {
			props.OnToggle()
		}
	})
	return Label(css.Class("sweep-check-row"),
		Attr("data-testid", "sweep-config-budget-"+props.ID),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("aria-label", props.Name),
			OnChange(onChange)}, checkedAttr(props.Checked)...)...),
		Span(css.Class("t-body"), props.Name),
	)
}
