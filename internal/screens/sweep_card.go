// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
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
func sweepBudgetCountPhrase(n int) string {
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
func budgetsSweepCard(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	hidden := ui.UseState("")
	_ = uistate.UseActiveScope().Get()
	memberID := uistate.ActiveMemberFromScope()
	var monthKey, totalStr, goalName, base, capacityStr string
	var amount int64
	var planBudgetIDs []string
	var capacityLimited bool
	nav := router.UseNavigate()
	onDismiss := ui.UseEvent(func() {
		markSweepPromptHandledForMember(monthKey, memberID)
		uistate.RequestPersist()
		hidden.Set(monthKey + ":" + memberID)
	})
	onSweep := ui.UseEvent(func() {
		if app == nil {
			return
		}
		if _, err := app.ApplyLeftoverSweep(uistate.SweepConfigGet().TargetGoalID, amount, base); err != nil {
			slog.Warn("apply leftover sweep", "err", err)
			uistate.PostNotice(uistate.T("sweep.applyError"), true)
			return
		}
		markSweepBudgetIDs(monthKey, planBudgetIDs, "swept")
		markSweepPromptHandledForMember(monthKey, memberID)
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("sweep.done", totalStr, goalName), false)
		hidden.Set(monthKey + ":" + memberID)
	})
	onReview := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/goals")) })
	reviewOnClick := OnClick(onReview)
	sweepOnClick := OnClick(onSweep)
	dismissOnClick := OnClick(onDismiss)
	if app == nil {
		return Fragment()
	}

	cfg := uistate.SweepConfigGet()
	now := time.Now()
	_, _, monthKey = closedMonthRange(now)
	if !sweepPromptDueForMember(monthKey, memberID, cfg) || hidden.Get() == monthKey+":"+memberID {
		return Fragment()
	}

	mStart, _, _ := closedMonthRange(now)
	input, err := app.BudgetViewInput(mStart)
	if err != nil {
		slog.Error("calculate month-close sweep", "err", err)
		return sweepHold(err)
	}
	// The live preferences are authoritative for the current view's member/grid.
	pr := uistate.CurrentPrefs()
	input.Period.WeekStart = pr.WeekStartWeekday()
	input.Period.PayCycleAnchor = budgetPayCycleAnchor(pr.PayCycleAnchor)
	input.PoolRollover = pr.BudgetRolloverLeftover
	base = input.Base
	budgets := app.Budgets()
	domainCfg := cfg.Domain()
	domainCfg.SweptBudgetIDs = sweepBudgetIDs(monthKey, domainCfg.BudgetIDs)
	plan, err := budgeting.EvaluateSweepWithMethodology(budgets, input, domainCfg, memberID,
		budgeting.Methodology(app.Settings().BudgetMethodology), app.SweepAllowedForGoal)
	if err != nil {
		slog.Error("calculate month-close sweep", "err", err)
		return sweepHold(err)
	}
	if !plan.HasLeftover() {
		return Fragment()
	}
	for _, line := range plan.Lines {
		planBudgetIDs = append(planBudgetIDs, line.BudgetID)
	}

	goalName = goalNameByID(app, cfg.TargetGoalID)
	dec := currency.Decimals(base)
	amount = plan.Total.Amount
	if !plan.Blocked && cfg.TargetGoalID != "" {
		capacity, capacityErr := app.LeftoverSweepCapacity(cfg.TargetGoalID, base)
		if capacityErr != nil {
			return sweepHold(capacityErr)
		}
		if capacity < amount {
			amount = capacity
			capacityLimited = true
			capacityStr = money.FormatMinor(amount, dec)
		}
	}
	totalStr = money.FormatMinor(amount, dec)

	// The primary action, or — when the target goal's account is over-earmarked
	// (XC7 gate) — a quiet explanation with a "Review goals" nudge instead.
	var action ui.Node
	if plan.Blocked {
		action = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mt3),
			P(css.Class("t-caption", tw.TextDim), uistate.T("sweep.blocked", goalName)),
			Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "sweep-review-goals"), reviewOnClick,
				uistate.T("integrity.reviewGoals")),
		)
	} else {
		action = Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "sweep-approve"),
				sweepOnClick,
				uistate.T("sweep.sweepAction", totalStr, goalName)),
			Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
				Attr("data-testid", "sweep-dismiss"), dismissOnClick,
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
					P(uistate.T("sweep.cardBody", totalStr, sweepBudgetCountPhrase(plan.BudgetCount()), goalName)),
					IfElse(capacityLimited, P(css.Class("t-caption", tw.TextDim), uistate.T("sweep.capacityLimited", capacityStr)), Fragment()),
				),
			),
			action,
		),
	)
}

// Household prompts retain the legacy state. Individual lenses keep independent
// handled months in app KV, so acting for one member cannot dismiss another's
// prompt, or erase the household's handled month.
func sweepPromptDueForMember(month, memberID string, cfg uistate.SweepConfig) bool {
	if memberID == "" {
		return uistate.SweepPromptDue(month, cfg)
	}
	return cfg.Enabled && month != "" && uistate.KVGet(sweepMemberPromptKey(memberID)) != month
}

func markSweepPromptHandledForMember(month, memberID string) {
	if memberID == "" {
		uistate.MarkSweepPromptHandled(month)
		return
	}
	uistate.KVSet(sweepMemberPromptKey(memberID), month)
}

func sweepMemberPromptKey(memberID string) string {
	return "cashflux:budgets:sweepprompt:member:" + memberID
}

func sweepBudgetStatusKey(month, budgetID string) string {
	return "cashflux:budgets:sweep:" + month + ":" + budgetID
}

func sweepBudgetIDs(month string, budgetIDs []string) []string {
	var done []string
	for _, id := range budgetIDs {
		status := uistate.KVGet(sweepBudgetStatusKey(month, id))
		if status == "swept" || status == "dismissed" {
			done = append(done, id)
		}
	}
	return done
}

func markSweepBudgetIDs(month string, budgetIDs []string, status string) {
	for _, id := range budgetIDs {
		if id != "" {
			uistate.KVSet(sweepBudgetStatusKey(month, id), status)
		}
	}
}

func sweepHold(err error) ui.Node {
	slog.Error("leftover sweep is unavailable", "err", err)
	return Div(css.Class("catchup-card"), Attr("role", "status"), Attr("data-testid", "sweep-hold"),
		Strong(uistate.T("sweep.cardTitle")), P(uistate.T("sweep.errorSafe")))
}

// budgetsSweepConfigModal renders the "Sweep leftovers" flip modal when its open
// atom is set, or Fragment() otherwise. It is rendered as a sibling of the bento
// (not inside a tile) so no tile transform breaks its centering.
func budgetsSweepConfigModal() ui.Node {
	openAtom := uistate.UseSweepConfigOpen()
	if !openAtom.Get() {
		return Fragment()
	}
	// Match the Add-budget standard: NoFooter + FlushBody so the form's .modal-scroll
	// body scrolls while its .modal-foot Save/Cancel bar stays pinned to the bottom.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("sweep.configTitle"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { openAtom.Set(false) },
		Back:      ui.CreateElement(sweepConfigForm, sweepConfigFormProps{OnDone: func() { openAtom.Set(false) }}),
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
	return ui.CreateElement(sweepConfigFormReady, sweepConfigFormReadyProps{Props: props, App: app})
}

type sweepConfigFormReadyProps struct {
	Props sweepConfigFormProps
	App   *appstate.App
}

func sweepConfigFormReady(ready sweepConfigFormReadyProps) ui.Node {
	props := ready.Props
	app := ready.App
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
			if selected.Get()[b.ID] && sweepBudgetEligible(b, app.Settings().BudgetMethodology) == "" {
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
	toggleEnabledOnChange := OnChange(onToggleEnabled)
	saveOnSubmit := OnSubmit(onSave)
	cancelOnClick := OnClick(onCancel)

	// Budget checkbox rows — each its own component (no On* in a loop). They sit in a
	// bordered inset list so the participation choices read as one aligned group.
	var budgetRows ui.Node
	if len(budgets) == 0 {
		budgetRows = P(css.Class("t-caption", tw.TextDim), uistate.T("sweep.configNoBudgets"))
	} else {
		rows := make([]ui.Node, 0, len(budgets))
		for _, b := range budgets {
			bid := b.ID
			reason := sweepBudgetEligible(b, app.Settings().BudgetMethodology)
			rows = append(rows, ui.CreateElement(sweepBudgetCheckRow, sweepBudgetRowProps{
				ID: bid, Name: b.Name, Checked: selected.Get()[bid] && reason == "", Disabled: reason != "", Reason: reason,
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

	// Standard config-modal shell (matches Add-budget): an .acct-edit-form with a
	// scrolling .modal-scroll body and a pinned .modal-foot Save/Cancel bar. The host
	// sets FlushBody so the foot stays put while the participation list scrolls.
	return Form(css.Class("acct-edit-form"), saveOnSubmit,
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("sweep.configIntro")),
			// Enable toggle as a standard bordered toggle row (matches the budget editor's
			// rollover row).
			Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2),
				Attr("style", "flex-wrap:nowrap;cursor:pointer"),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"),
					Attr("data-testid", "sweep-config-enable"), toggleEnabledOnChange},
					checkedAttr(enabled.Get())...)...),
				Span(css.Class("t-body"), uistate.T("sweep.configEnable")),
			),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
				Span(css.Class("t-caption", tw.TextDim), uistate.T("sweep.configBudgets")),
				budgetRows,
			),
			IfElse(uistate.CurrentPrefs().BudgetRolloverLeftover,
				P(css.Class("t-caption", tw.TextDim), uistate.T("sweep.poolRolloverOff")), Fragment()),
			labeledField(uistate.T("sweep.configGoal"), goalPicker),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"),
				Attr("data-testid", "sweep-config-cancel"), cancelOnClick, uistate.T("sweep.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"),
				Attr("data-testid", "sweep-config-save"), uistate.T("sweep.save")),
		),
	)
}

type sweepBudgetRowProps struct {
	ID       string
	Name     string
	Checked  bool
	Disabled bool
	Reason   string
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
	var disabled []any
	if props.Disabled {
		disabled = append(disabled, Attr("disabled", "disabled"), Attr("aria-disabled", "true"))
	}
	inputArgs := []any{css.Class("cf-check"), Type("checkbox"), Attr("aria-label", props.Name), OnChange(onChange)}
	inputArgs = append(inputArgs, disabled...)
	inputArgs = append(inputArgs, checkedAttr(props.Checked)...)
	return Label(css.Class("sweep-check-row"),
		Attr("data-testid", "sweep-config-budget-"+props.ID),
		Input(inputArgs...),
		Span(css.Class("t-body"), props.Name, IfElse(props.Reason != "", Span(css.Class("t-caption", tw.TextDim), " — ", props.Reason), Fragment())),
	)
}

func sweepBudgetEligible(b domain.Budget, householdMethod string) string {
	method := budgeting.ParseMethodology(householdMethod)
	if b.Methodology != "" && budgeting.Methodology(b.Methodology).Valid() {
		method = budgeting.Methodology(b.Methodology)
	}
	switch {
	case b.IsSaving():
		return uistate.T("sweep.ineligibleSaving")
	case b.Rollover:
		return uistate.T("sweep.ineligibleRollover")
	case b.Period != domain.PeriodMonthly:
		return uistate.T("sweep.ineligiblePeriod")
	case method == budgeting.MethodEnvelope:
		return uistate.T("sweep.ineligibleEnvelope")
	default:
		return ""
	}
}
