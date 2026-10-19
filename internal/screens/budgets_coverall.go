// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/coverplan"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// coverAllFeatureCode is the SMART-B14 catalog code that gates the cover-overages
// feature (Free, on by default, opt-out under the Smart settings).
const coverAllFeatureCode = "SMART-B14"

// Keep the existing private test seam while the implementation lives in pure Go.
const coverAllNextSource = coverplan.NextSource

type coverAllOver = coverplan.Over
type coverAllSource = coverplan.Source

var (
	coverAllOpener                js.Value
	coverAllSourceLess            = coverplan.SourceLess
	coverAllMonthlyRoom           = coverplan.MonthlyRoom
	coverAllMonthlyEquivalent     = coverplan.MonthlyEquivalent
	coverAllCoveredAt             = coverplan.CoveredAt
	coverAllDefaultSource         = coverplan.DefaultSource
	coverAllSourcesFor            = coverplan.SourcesFor
	coverAllHasDonor              = coverplan.HasDonor
	coverAllSelectedTotal         = coverplan.SelectedTotal
	coverAllSelectedTotalChecked  = coverplan.SelectedTotalChecked
	coverAllHasNextMonthSelection = coverplan.HasNextMonthSelection
)

// coverAllData adapts the grid's evaluated statuses and the application's ledger
// input to the pure Cover-all read model. Writes always target the viewed period.
func coverAllData(app *appstate.App, member string, vw period.Window, pr prefs.Prefs, _ bool) ([]coverAllOver, []coverAllSource, string) {
	v := computeBudgetView(app, member, vw, pr, false)
	base := budgetBaseCurrency(v.Base)
	start, end := vw.Range()
	viewPeriod := budgeting.ResolveViewPeriod(start, end, time.Now(), pr.WeekStartWeekday(), budgetPayCycleAnchor(pr.PayCycleAnchor))
	input, err := app.BudgetViewInput(viewPeriod.Anchor)
	if err != nil {
		return nil, nil, base
	}
	overs, sources := coverplan.Data(v.Statuses, input, viewPeriod)
	return overs, sources, base
}

// coverAllButtonProps carries nothing — the button reads shared state itself.
type coverAllButtonProps struct{}

// coverAllBannerButton is the over-banner entry point that opens the cover-overages
// modal. Its own component so its click hook stays at a stable position; the CALLER
// gates it on SMART-B14 + an over-count so this only mounts when there's work to do.
func coverAllBannerButton(_ coverAllButtonProps) ui.Node {
	open := uistate.UseCoverAllOpen()
	onOpen := ui.UseEvent(Prevent(func() {
		coverAllOpener = js.Global().Get("document").Get("activeElement")
		open.Set(true)
	}))
	return Button(css.Class("btn btn-tool cover-all-open", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-cover-all"), Title(uistate.T("bv7g.cover.title")), OnClick(onOpen),
		uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("coverAll.button")))
}

// budgetsCoverAllModal renders the cover-overages flip modal when its atom is set,
// as a shell-root sibling of the bento (no tile transform clips it). Matches the
// Add-budget footer standard (NoFooter + FlushBody + the form's pinned .modal-foot).
func budgetsCoverAllModal() ui.Node {
	open := uistate.UseCoverAllOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("bv7g.cover.title"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { closeCoverAll(func() { open.Set(false) }) },
		Back:      ui.CreateElement(coverAllForm, coverAllFormProps{OnDone: func() { closeCoverAll(func() { open.Set(false) }) }}),
	})
}

func closeCoverAll(close func()) {
	close()
	opener := coverAllOpener
	if !opener.Truthy() || opener.Get("focus").Type() != js.TypeFunction {
		return
	}
	var restore js.Func
	restore = js.FuncOf(func(js.Value, []js.Value) any {
		restore.Release()
		if opener.Get("isConnected").Bool() {
			opener.Call("focus")
		}
		return nil
	})
	js.Global().Call("setTimeout", restore, 0)
}

type coverAllFormProps struct {
	OnDone   func()
	TargetID string
}

// coverAllForm binds shared application state to the pure read model and named
// rendering children. Event hooks stay at stable positions within their owner.
func coverAllForm(props coverAllFormProps) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	member := uistate.UseActiveMember().Get()
	pr := uistate.UsePrefs().Get()
	vw := uistate.UsePeriod().Get()
	showLM := uistate.UseBudgetsLastMonth().Get()
	choices := ui.UseState(map[string]string{})
	errText := ui.UseState("")
	checkpointSaving := ui.UseState(false)
	seededSig := ui.UseState("")
	var overs []coverAllOver
	var sources []coverAllSource
	base := budgetBaseCurrency("")
	if app != nil {
		overs, sources, base = coverAllData(app, member, vw, pr, showLM)
	}
	model := coverplan.Prepare(overs, sources, base, props.TargetID)
	if seed, reset := coverplan.SeedChoices(model, seededSig.Get()); reset {
		choices.Set(seed)
		errText.Set("")
		seededSig.Set(model.Signature)
	}
	setChoice := func(id, value string) { choices.Set(coverplan.Choose(choices.Get(), id, value)) }
	actions := coverAllActions{IsSaving: checkpointSaving.Get, SetSaving: checkpointSaving.Set, SetError: errText.Set, OnDone: props.OnDone}
	apply := ui.UseEvent(Prevent(func() { applyCoverAll(app, model, choices.Get(), vw, actions) }))
	display := coverplan.Present(model, choices.Get(), checkpointSaving.Get(), uistate.T, fmtMoney)
	return Form(css.Class("acct-edit-form cover-all"), OnSubmit(apply),
		ui.CreateElement(coverAllBody, coverAllBodyProps{
			Model: model, Display: display, Chosen: choices.Get(), Error: errText.Get(), OnChoice: setChoice,
		}),
		ui.CreateElement(coverAllFooter, coverAllFooterProps{
			Empty: model.Empty, Saving: checkpointSaving.Get(), Disabled: display.Disabled,
			ApplyLabel: display.ApplyLabel, OnDone: props.OnDone,
		}),
	)
}

type coverAllActions struct {
	IsSaving  func() bool
	SetSaving func(bool)
	SetError  func(string)
	OnDone    func()
}

// applyCoverAll owns the checkpoint/write effects; the request is validated by
// pure logic before any checkpoint is scheduled, as in the original form.
func applyCoverAll(app *appstate.App, model coverplan.Model, chosen map[string]string, vw period.Window, actions coverAllActions) {
	if !model.CanStart(app != nil, actions.IsSaving()) {
		return
	}
	requests, problem := coverplan.Submission(model, chosen)
	if problem != nil {
		actions.SetError(coverplan.ProblemText(problem, uistate.T, fmtMoney, budgetErrorText))
		return
	}
	actions.SetSaving(true)
	uistate.SaveCheckpointThen(coverplan.CheckpointText(model, uistate.T), func(_ string, checkpointErr error) {
		actions.SetSaving(false)
		if checkpointErr != nil {
			uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
			return
		}
		completeCoverAll(app, model, chosen, requests, vw, actions)
	})
}

func completeCoverAll(app *appstate.App, model coverplan.Model, chosen map[string]string, requests []budgeting.CoverRequest, vw period.Window, actions coverAllActions) {
	completion := coverplan.Complete(model, chosen, requests, uistate.T, fmtMoney)
	if completion.Empty {
		actions.SetError(completion.Text)
		return
	}
	// The existing application boundary re-evaluates room and commits every
	// debit/credit atomically through budgeting.PlanCover after the checkpoint.
	if err := app.CoverBudgetsAt(requests, budgetViewAnchor(vw, budgetNow())); err != nil {
		slog.Error("budgets: cover-all write rejected", "err", err)
		actions.SetError(budgetErrorText(err))
		return
	}
	uistate.RequestPersist()
	uistate.BumpDataRevision()
	uistate.PostNotice(completion.Text, false)
	if actions.OnDone != nil {
		actions.OnDone()
	}
}

type coverAllBodyProps struct {
	Model    coverplan.Model
	Display  coverplan.Display
	Chosen   map[string]string
	Error    string
	OnChoice func(string, string)
}

func coverAllBody(props coverAllBodyProps) ui.Node {
	return Div(css.Class("modal-scroll"),
		If(props.Model.Empty, P(css.Class("empty cover-all-empty"), Attr("data-testid", "cover-all-empty"), uistate.T("coverAll.none"))),
		If(!props.Model.Empty, Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				Attr("data-testid", "cover-all-intro"), props.Display.Intro),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cover-all-impact"),
				Style(map[string]string{"margin": "0"}), uistate.T(props.Display.ImpactKey)),
			ui.CreateElement(coverAllList, coverAllListProps{
				Model: props.Model, Chosen: props.Chosen, OnChoice: props.OnChoice,
			}),
			If(props.Display.NextMonthHint, P(css.Class("t-caption", tw.TextDim, "cover-all-hint"), uistate.T("bv6g.cover.nextMonthHint"))),
			If(props.Model.CurrencyError, P(css.Class("err"), Attr("role", "alert"), Attr("data-testid", "cover-all-currency-error"), uistate.T("coverAll.currencyError"))),
			If(props.Error != "", P(css.Class("err"), Attr("role", "alert"), props.Error)),
		)),
	)
}

type coverAllListProps struct {
	Model    coverplan.Model
	Chosen   map[string]string
	OnChoice func(string, string)
}

func coverAllList(props coverAllListProps) ui.Node {
	return Div(css.Class("cover-all-list"),
		MapKeyed(props.Model.Overs, func(over coverAllOver) any { return over.ID }, func(over coverAllOver) ui.Node {
			return ui.CreateElement(coverAllRow, coverAllRowProps{
				Over: over, Sources: props.Model.Sources, Dec: props.Model.Decimals, Selected: props.Chosen[over.ID],
				OnChange: func(value string) { props.OnChoice(over.ID, value) },
			})
		}),
	)
}

type coverAllFooterProps struct {
	Empty, Saving, Disabled bool
	ApplyLabel              string
	OnDone                  func()
}

func coverAllFooter(props coverAllFooterProps) ui.Node {
	onCancel := ui.UseEvent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	})
	return Div(css.Class("modal-foot"),
		IfElse(props.Empty,
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "cover-all-close"), OnClick(onCancel), uistate.T("action.close")),
			Fragment(
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "cover-all-cancel"), OnClick(onCancel), uistate.T("coverAll.cancel")),
				buttonWithDisabled(props.Disabled, []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "cover-all-apply")}, IfElse(props.Saving, Text(uistate.T("enc34.saving")), Text(props.ApplyLabel))),
			)),
	)
}

type coverAllRowProps struct {
	Over     coverAllOver
	Sources  []coverAllSource
	Dec      int
	Selected string
	OnChange func(string)
}

// coverAllRow is one over-budget line: its name + overage, and a source picker. Own
// component so the select's change hook sits at a stable position (never in a loop).
func coverAllRow(props coverAllRowProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) { props.OnChange(e.GetValue()) })
	o := props.Over
	opts := []any{css.Class("field cover-all-src"), Attr("data-testid", "cover-all-src-"+o.ID),
		Attr("aria-label", uistate.T("coverAll.sourceLabel")), OnChange(onChange)}
	opts = append(opts,
		Option(Value(""), SelectedIf(props.Selected == ""), uistate.T("coverAll.sourceSkip")),
		If(!o.Rollover, Option(Value(coverAllNextSource), SelectedIf(props.Selected == coverAllNextSource), uistate.T("coverAll.sourceNextMonth"))))
	for _, s := range coverAllSourcesFor(o, props.Sources) {
		opts = append(opts, Option(Value(s.ID), SelectedIf(props.Selected == s.ID),
			uistate.T("bv7g.cover.sourceBudget", s.Name, fmtMoney(s.Avail), fmtMoney(s.Monthly))))
	}
	return Div(css.Class("cover-all-row"), Attr("data-testid", "cover-all-row"),
		Div(css.Class("cover-all-row-main"),
			Span(css.Class("cover-all-row-name"), o.Name),
			Span(css.Class("cover-all-row-over", tw.TextDown), uistate.T("coverAll.overBy", fmtMoney(o.Shortfall)))),
		Div(css.Class("cover-all-row-src"),
			Span(css.Class("cover-all-row-srclabel", tw.Text12, tw.TextDim), uistate.T("coverAll.sourceLabel")),
			Select(opts...)))
}
