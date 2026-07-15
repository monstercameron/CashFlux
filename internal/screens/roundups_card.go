// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/roundups"
	"github.com/monstercameron/CashFlux/internal/savings"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// roundups_card.go is the UI shell for the virtual round-up ritual (TX11): the
// quiet cadence-boundary card on /goals, the "Round-ups" config flip modal from
// the goals toolbar, and the running-jar figure on the target goal's row. All
// computation lives in the tested roundups.Accrue; this file only renders.

// roundUpWindow returns the accrual window (since, now] and the cadence period
// key for the current cadence, given the config's last-sweep stamp. When no
// sweep has run yet, the window starts at the beginning of the current cadence
// period so the first card reflects this period's spare change.
func roundUpWindow(cfg uistate.RoundUpConfig, now time.Time) (since, end time.Time, periodKey string) {
	cadence := cfg.EffectiveCadence()
	periodKey = savings.PeriodKey(now, cadence)
	var periodStart time.Time
	if cadence == uistate.RoundUpCadenceMonthly {
		periodStart, _ = dateutil.MonthRange(now)
	} else {
		periodStart = dateutil.WeekStart(now, uistate.LoadPrefs().WeekStartWeekday())
	}
	return cfg.SinceOr(periodStart), now, periodKey
}

// roundUpGoalName looks up the target goal's name, falling back to a generic label.
func roundUpGoalName(app *appstate.App, id string) string {
	for _, g := range app.Goals() {
		if g.ID == id {
			return g.Name
		}
	}
	return uistate.T("roundups.fallbackGoal")
}

// cadencePhrase renders "this week" / "this month".
func cadencePhrase(cadence string) string {
	if cadence == uistate.RoundUpCadenceMonthly {
		return uistate.T("roundups.thisMonth")
	}
	return uistate.T("roundups.thisWeek")
}

// goalsRoundUpCard renders the dismissible round-up sweep card, or Fragment()
// when the ritual is off, no goal is set, nothing accrued, or this cadence period
// was already handled. One sentence, one primary action, one dismiss — never
// naggy. Its own component so the dismissal hook stays at a stable position.
func goalsRoundUpCard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	cfg := uistate.RoundUpConfigGet()
	now := time.Now()
	since, end, periodKey := roundUpWindow(cfg, now)

	hidden := ui.UseState(false)
	onDismiss := ui.UseEvent(func() {
		uistate.MarkRoundUpPromptHandled(periodKey)
		uistate.RequestPersist()
		hidden.Set(true)
	})

	acc := roundups.Accrue(app.Transactions(), cfg.ParticipatingSet(), app.TxnLinks(), since, end)
	goalName := roundUpGoalName(app, cfg.TargetGoalID)
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	cur := acc.Currency
	if cur == "" {
		cur = base
	}
	dec := currency.Decimals(cur)
	totalStr := currency.Symbol(cur) + money.FormatMinor(acc.TotalCents, dec)

	onApprove := ui.UseEvent(func() {
		if cfg.TargetGoalID == "" {
			uistate.PostNotice(uistate.T("roundups.needGoal"), true)
			return
		}
		if _, err := app.ApplyAllocation([]allocate.Action{{
			Kind:            allocate.GoalContribution,
			DestinationID:   cfg.TargetGoalID,
			DestinationName: goalName,
			Amount:          acc.TotalCents,
		}}); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.StampRoundUpSweep(now)
		uistate.MarkRoundUpPromptHandled(periodKey)
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("roundups.done", totalStr, goalName), false)
		hidden.Set(true)
	})

	// Gate: enabled, a goal chosen, spare change accrued, and not already handled
	// for this cadence period, and not locally hidden this render.
	if !cfg.Enabled || cfg.TargetGoalID == "" || !acc.HasSpareChange() ||
		uistate.RoundUpPromptHandledPeriod() == periodKey || hidden.Get() {
		return Fragment()
	}

	return Div(
		css.Class("catchup-card"),
		Attr("role", "complementary"),
		Attr("data-testid", "roundups-card"),
		Attr("aria-label", uistate.T("roundups.cardTitle")),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), "🪙"),
				Div(css.Class("catchup-card-text"),
					Strong(uistate.T("roundups.cardTitle")),
					P(uistate.T("roundups.cardBody", totalStr, cadencePhrase(cfg.EffectiveCadence()), goalName)),
				),
			),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-primary btn-sm"), Type("button"),
					Attr("data-testid", "roundups-approve"), OnClick(onApprove),
					uistate.T("roundups.addAction", totalStr, goalName)),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "roundups-dismiss"), OnClick(onDismiss),
					uistate.T("roundups.dismiss")),
			),
		),
	)
}

// goalRoundUpJar returns the running-jar caption ("$6.37 in round-ups") for the
// target goal's row and ok=true, or (Fragment(), false) when round-ups are off,
// this is not the target goal, or nothing has accrued. Callers render it only
// when ok so an empty row line is never emitted. Pure read; no hooks (safe inside
// a row).
func goalRoundUpJar(app *appstate.App, goalID string) (ui.Node, bool) {
	if app == nil {
		return Fragment(), false
	}
	cfg := uistate.RoundUpConfigGet()
	if !cfg.Enabled || cfg.TargetGoalID == "" || cfg.TargetGoalID != goalID {
		return Fragment(), false
	}
	now := time.Now()
	since, end, _ := roundUpWindow(cfg, now)
	acc := roundups.Accrue(app.Transactions(), cfg.ParticipatingSet(), app.TxnLinks(), since, end)
	if !acc.HasSpareChange() {
		return Fragment(), false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	cur := acc.Currency
	if cur == "" {
		cur = base
	}
	totalStr := currency.Symbol(cur) + money.FormatMinor(acc.TotalCents, currency.Decimals(cur))
	return Span(css.Class("budget-sub", tw.TextDim), Attr("data-testid", "goal-roundup-jar-"+goalID),
		uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Span(uistate.T("roundups.jar", totalStr))), true
}

// roundupConfigToolbarButton is the "Round-ups" btn-tool for the goals toolbar.
// It opens the config flip modal (rendered from the surface root). Its own
// component so its click hook stays at a stable position.
func roundupConfigToolbarButton() ui.Node {
	openAtom := uistate.UseRoundUpConfigOpen()
	onOpen := ui.UseEvent(Prevent(func() { openAtom.Set(true) }))
	return Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "goals-roundup-config"), Title(uistate.T("roundups.openConfig")),
		OnClick(onOpen),
		uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("roundups.openConfig")))
}

// goalsRoundUpConfigModal renders the "Round-ups" flip modal when its open atom
// is set, or Fragment() otherwise. Rendered as a sibling of the bento (not inside
// a tile) so no tile transform breaks its centering.
func goalsRoundUpConfigModal() ui.Node {
	openAtom := uistate.UseRoundUpConfigOpen()
	if !openAtom.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("roundups.configTitle"),
		Width:    uiw.FlipMediumW,
		Height:   "min(90vh, 600px)",
		NoFooter: true,
		OnClose:  func() { openAtom.Set(false) },
		Back:     ui.CreateElement(roundupConfigForm, roundupConfigFormProps{OnDone: func() { openAtom.Set(false) }}),
	})
}

type roundupConfigFormProps struct {
	OnDone func()
}

// roundupConfigForm is the staged Save/Cancel config body: enable toggle, target
// goal, cadence, and participating accounts. Edits are held in local draft state
// and only committed on Save (staged, like the sweep config).
func roundupConfigForm(props roundupConfigFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()
	cfg := uistate.RoundUpConfigGet()

	enabled := ui.UseState(cfg.Enabled)
	goalID := ui.UseState(cfg.TargetGoalID)
	cadence := ui.UseState(cfg.EffectiveCadence())
	seed := map[string]bool{}
	for _, id := range cfg.AccountIDs {
		seed[id] = true
	}
	selected := ui.UseState(seed)

	onToggleEnabled := ui.UseEvent(func() { enabled.Set(!enabled.Get()) })
	onGoal := ui.UseEvent(func(e ui.Event) { goalID.Set(e.GetValue()) })
	onCadence := ui.UseEvent(func(e ui.Event) { cadence.Set(e.GetValue()) })

	goals := app.Goals()
	accounts := app.Accounts()

	onSave := ui.UseEvent(func() {
		ids := make([]string, 0, len(selected.Get()))
		for _, a := range accounts { // stable, store order
			if selected.Get()[a.ID] {
				ids = append(ids, a.ID)
			}
		}
		next := uistate.RoundUpConfigGet()
		next.Enabled = enabled.Get()
		next.TargetGoalID = goalID.Get()
		next.Cadence = cadence.Get()
		next.AccountIDs = ids
		uistate.SetRoundUpConfig(next)
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	})
	onCancel := ui.UseEvent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	})

	// Goal picker.
	var goalPicker ui.Node
	if len(goals) == 0 {
		goalPicker = P(css.Class("t-caption", tw.TextDim), uistate.T("roundups.configNoGoals"))
	} else {
		opts := make([]ui.Node, 0, len(goals)+1)
		opts = append(opts, Option(Value(""), SelectedIf(goalID.Get() == ""), uistate.T("roundups.configGoalNone")))
		for _, g := range goals {
			opts = append(opts, Option(Value(g.ID), SelectedIf(goalID.Get() == g.ID), g.Name))
		}
		goalPicker = Select(css.Class("field"), Attr("data-testid", "roundups-config-goal"),
			Attr("aria-label", uistate.T("roundups.configGoal")), OnChange(onGoal), opts)
	}

	cadencePicker := Select(css.Class("field"), Attr("data-testid", "roundups-config-cadence"),
		Attr("aria-label", uistate.T("roundups.configCadence")), OnChange(onCadence),
		Option(Value(uistate.RoundUpCadenceWeekly), SelectedIf(cadence.Get() == uistate.RoundUpCadenceWeekly), uistate.T("roundups.cadenceWeekly")),
		Option(Value(uistate.RoundUpCadenceMonthly), SelectedIf(cadence.Get() == uistate.RoundUpCadenceMonthly), uistate.T("roundups.cadenceMonthly")),
	)

	// Account participation rows — each its own component (no On* in a loop).
	var accountRows ui.Node
	if len(accounts) == 0 {
		accountRows = P(css.Class("t-caption", tw.TextDim), uistate.T("roundups.configNoAccounts"))
	} else {
		rows := make([]ui.Node, 0, len(accounts))
		for _, a := range accounts {
			aid := a.ID
			rows = append(rows, ui.CreateElement(sweepBudgetCheckRow, sweepBudgetRowProps{
				ID: "acct-" + aid, Name: a.Name, Checked: selected.Get()[aid],
				OnToggle: func() {
					nextSel := map[string]bool{}
					for k, v := range selected.Get() {
						nextSel[k] = v
					}
					nextSel[aid] = !nextSel[aid]
					selected.Set(nextSel)
				},
			}))
		}
		accountRows = Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), rows)
	}

	return Div(css.Class("modal-scroll", tw.Flex, tw.FlexCol, tw.Gap3),
		P(css.Class("t-caption", tw.TextDim), uistate.T("roundups.configIntro")),
		Label(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"),
				Attr("data-testid", "roundups-config-enable"), OnChange(onToggleEnabled)},
				checkedAttr(enabled.Get())...)...),
			Span(css.Class("t-body"), uistate.T("roundups.configEnable")),
		),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Span(css.Class("t-caption", tw.TextDim), uistate.T("roundups.configGoal")),
			goalPicker,
		),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Span(css.Class("t-caption", tw.TextDim), uistate.T("roundups.configCadence")),
			cadencePicker,
		),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Span(css.Class("t-caption", tw.TextDim), uistate.T("roundups.configAccounts")),
			P(css.Class("t-caption", tw.TextDim), uistate.T("roundups.configAllAccounts")),
			accountRows,
		),
		Div(css.Class("modal-foot", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
				Attr("data-testid", "roundups-config-cancel"), OnClick(onCancel), uistate.T("roundups.cancel")),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "roundups-config-save"), OnClick(onSave), uistate.T("roundups.save")),
		),
	)
}
