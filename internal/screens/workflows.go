// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workflow"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Workflows is the automation manager, rebuilt as a from-scratch "automations
// desk" (2026-07-04): a masthead, the three savings quick-starts as a panel
// band (C183/C184/C185/C188 — still the most likely first action), then the
// automation registry (ledger rows: status dot, name, trigger · condition ·
// actions, dry-run-first controls, ⋯ menu with a two-step delete) beside the
// workflow composer whose footprint reads the draft back in plain English.
// Planning is the pure internal/workflow engine; applying effects + recording
// runs is appstate — mechanics unchanged.
func Workflows() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	rev := ui.UseState(0)
	revVal := rev.Get()
	refresh := func() { rev.Set(rev.Get() + 1) }

	wfs := app.Workflows()
	rows := MapKeyed(wfs,
		func(w workflow.Workflow) any { return w.ID },
		func(w workflow.Workflow) ui.Node {
			return ui.CreateElement(workflowRow, workflowRowProps{Workflow: w, Refresh: refresh})
		},
	)
	var registryBody ui.Node = P(css.Class("wf-empty"), uistate.T("wfs.emptyRegistry"))
	if len(wfs) > 0 {
		registryBody = Div(css.Class("wf-rows"), rows)
	}

	masthead := Div(css.Class("wman-head"),
		Span(css.Class("studio-eyebrow"), uistate.T("wfs.eyebrow")),
		H2(css.Class("studio-design-title"), uistate.T("wfs.title")),
		P(css.Class("studio-design-sub"), uistate.T("wfs.lede")),
	)

	// Savings quick-starts: the three auto-save templates as one panel band.
	quick := Div(css.Class("wf-quick"),
		H3(css.Class("wf-sec-title"), uistate.T("workflows.savingsTitle")),
		P(css.Class("wf-sec-lede"), uistate.T("workflows.savingsDesc")),
		Div(css.Class("wf-quick-grid"),
			ui.CreateElement(pyfForm, pyfFormProps{Refresh: refresh}),
			ui.CreateElement(sweepForm, sweepFormProps{Refresh: refresh}),
			ui.CreateElement(roundUpForm, roundUpFormProps{Refresh: refresh}),
		),
	)

	registryHead := Div(css.Class("wf-sec-head"),
		H3(css.Class("wf-sec-title"), uistate.T("workflows.yours")),
		If(len(wfs) > 0, Span(css.Class("wf-count"), strconv.Itoa(len(wfs)))),
	)

	return Div(css.Class("wf-deck"),
		masthead,
		quick,
		Div(css.Class("wf-grid"),
			Div(css.Class("wf-main"),
				registryHead,
				registryBody,
				// Rev rides along so the component's props CHANGE on refresh —
				// with empty props the framework memoizes it (reference-equal
				// props) and a just-completed run wouldn't appear until re-nav.
				ui.CreateElement(workflowHistory, workflowHistoryProps{Rev: revVal}),
			),
			Div(css.Class("wf-aside"),
				ui.CreateElement(addWorkflowForm, addWorkflowFormProps{Refresh: refresh}),
			),
		),
	)
}

// pyfFormProps passes the refresh callback into the pay-yourself-first form.
type pyfFormProps struct{ Refresh func() }

// pyfEntry summarizes one pay-yourself-first workflow for the quick-start
// panel's "Already set up" block.
type pyfEntry struct {
	From, To, Amount, Cadence string
	FromID, ToID              string
	Cad                       domain.RecurringCadence
	Enabled                   bool
}

// pyfActive lists the existing pay-yourself-first workflows — scheduled
// single-transfer workflows minted by the quick-start, recognized by their
// "pyf:"-prefixed transfer dedupe key — INCLUDING disabled ones, so the panel
// reflects what's already set up instead of presenting an eternally blank
// form, and so a duplicate of a merely-off transfer (which would silently
// double the money moved once re-enabled) can be caught at save.
func pyfActive(app *appstate.App, dec int) []pyfEntry {
	names := map[string]string{}
	for _, ac := range app.Accounts() {
		names[ac.ID] = ac.Name
	}
	nameOf := func(id string) string {
		if n := names[id]; n != "" {
			return n
		}
		return id
	}
	var out []pyfEntry
	for _, w := range app.Workflows() {
		if len(w.Actions) != 1 {
			continue
		}
		a := w.Actions[0]
		if a.Kind != workflow.ActionTransfer || !strings.HasPrefix(a.DedupeKey, "pyf:") {
			continue
		}
		cadLabel := uistate.T("workflows.pyfCadenceMonthly")
		if w.Trigger.Cadence == domain.CadenceWeekly {
			cadLabel = uistate.T("workflows.pyfCadenceWeekly")
		}
		out = append(out, pyfEntry{
			From: nameOf(a.TransferFromAccountID), To: nameOf(a.TransferToAccountID),
			Amount:  money.FormatMinor(a.TransferAmount, dec),
			Cadence: cadLabel,
			FromID:  a.TransferFromAccountID, ToID: a.TransferToAccountID, Cad: w.Trigger.Cadence,
			Enabled: w.Enabled,
		})
	}
	return out
}

// pyfForm is the savings automations section (C188) with the pay-yourself-first
// template (C185). It creates a scheduled ActionTransfer workflow so money moves
// from a source account to a savings account on the chosen cadence — a real
// two-leg transfer, not a single-leg autopost.
//
// Its own component so all UseState/UseEvent hooks sit at stable render positions,
// satisfying the framework's no-hooks-in-loops rule even when the account list
// changes length.
func pyfForm(props pyfFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)

	fromID := ui.UseState("")
	toID := ui.UseState("")
	amtStr := ui.UseState("")
	cadence := ui.UseState(string(domain.CadenceMonthly))
	msg := ui.UseState("")
	success := ui.UseState(false)
	// When a PYF transfer is already running, the panel opens as an "Already
	// running" summary; the form stays a click away behind "Add another".
	showForm := ui.UseState(false)

	onFrom := ui.UseEvent(func(e ui.Event) { fromID.Set(e.GetValue()); msg.Set(""); success.Set(false) })
	onTo := ui.UseEvent(func(e ui.Event) { toID.Set(e.GetValue()); msg.Set(""); success.Set(false) })
	onAmt := ui.UseEvent(func(v string) { amtStr.Set(v); msg.Set(""); success.Set(false) })
	onCadence := ui.UseEvent(func(e ui.Event) { cadence.Set(e.GetValue()) })
	onShowForm := ui.UseEvent(Prevent(func() { showForm.Set(true); msg.Set(""); success.Set(false) }))

	active := pyfActive(app, dec)

	save := func() {
		app := appstate.Default
		from := fromID.Get()
		to := toID.Get()
		if from == "" {
			msg.Set(uistate.T("workflows.pyfNeedFrom"))
			return
		}
		if to == "" {
			msg.Set(uistate.T("workflows.pyfNeedTo"))
			return
		}
		if from == to {
			msg.Set(uistate.T("workflows.pyfSameAccount"))
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(amtStr.Get()), dec)
		if err != nil || amt <= 0 {
			msg.Set(uistate.T("workflows.pyfNeedAmount"))
			return
		}
		cad := domain.RecurringCadence(cadence.Get())
		// Refuse a duplicate on the same route + schedule: a second scheduled
		// workflow would silently double the money moved — and a duplicate of a
		// merely-OFF transfer would do the same the day it's re-enabled.
		for _, e := range pyfActive(app, dec) {
			if e.FromID == from && e.ToID == to && e.Cad == cad {
				if e.Enabled {
					msg.Set(uistate.T("wfs.pyfDuplicate"))
				} else {
					msg.Set(uistate.T("wfs.pyfDuplicateOff"))
				}
				return
			}
		}
		if _, err := app.CreatePayYourselfFirstWorkflow(from, to, amt, cad); err != nil {
			msg.Set(err.Error())
			return
		}
		// Reset form on success and fold back to the summary.
		fromID.Set("")
		toID.Set("")
		amtStr.Set("")
		cadence.Set(string(domain.CadenceMonthly))
		msg.Set("")
		success.Set(true)
		showForm.Set(false)
		if props.Refresh != nil {
			props.Refresh()
		}
	}

	// Build account option lists for from/to selects. All non-archived accounts
	// are offered; the user chooses the appropriate source and destination.
	fromNone := Option(Value(""), SelectedIf(fromID.Get() == ""), uistate.T("workflows.pyfChooseAccount"))
	toNone := Option(Value(""), SelectedIf(toID.Get() == ""), uistate.T("workflows.pyfChooseAccount"))
	var fromOpts, toOpts []ui.Node
	fromOpts = append(fromOpts, fromNone)
	toOpts = append(toOpts, toNone)
	for _, ac := range app.Accounts() {
		if ac.Archived {
			continue
		}
		label := ac.Name + " (" + string(ac.Type) + ")"
		fromOpts = append(fromOpts, Option(Value(ac.ID), SelectedIf(fromID.Get() == ac.ID), label))
		toOpts = append(toOpts, Option(Value(ac.ID), SelectedIf(toID.Get() == ac.ID), label))
	}

	amtPlaceholder := uistate.T("workflows.pyfAmount", base)

	// "Already set up" summary — one line per PYF transfer, OFF ones dimmed.
	activeLines := make([]ui.Node, 0, len(active))
	for _, e := range active {
		lineCls := "wf-active-line"
		dotCls := "wf-dot"
		if !e.Enabled {
			lineCls += " is-off"
			dotCls += " is-off"
		}
		activeLines = append(activeLines, Div(ClassStr(lineCls),
			Span(ClassStr(dotCls), Attr("aria-hidden", "true")),
			Span(e.From+" → "+e.To+" · "+e.Amount+" "+base+" · "+e.Cadence),
			If(!e.Enabled, Span(css.Class("wman-hidden-tag"), uistate.T("wfs.disabledTag"))),
		))
	}
	activeBlock := Div(append([]any{css.Class("wf-active"), Attr("data-testid", "pyf-active"),
		Span(css.Class("fld-foot-title"), uistate.T("wfs.pyfActiveTitle"))},
		append(anyNodes(activeLines), P(css.Class("wf-hint"), uistate.T("wfs.pyfActiveHint")))...)...)

	formVisible := len(active) == 0 || showForm.Get()
	form := Fragment(
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.pyfFrom")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.pyfFrom")), OnChange(onFrom), fromOpts),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.pyfTo")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.pyfTo")), OnChange(onTo), toOpts),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), amtPlaceholder),
			Input(css.Class("field"), Attr("placeholder", amtPlaceholder), Attr("aria-label", amtPlaceholder),
				Attr("inputmode", "decimal"), Value(amtStr.Get()), OnInput(onAmt)),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.pyfCadence")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.pyfCadence")), OnChange(onCadence),
				Option(Value(string(domain.CadenceWeekly)), SelectedIf(cadence.Get() == string(domain.CadenceWeekly)), uistate.T("workflows.pyfCadenceWeekly")),
				Option(Value(string(domain.CadenceMonthly)), SelectedIf(cadence.Get() == string(domain.CadenceMonthly)), uistate.T("workflows.pyfCadenceMonthly")),
			),
		),
	)

	return Div(css.Class("wf-quick-panel"),
		H4(css.Class("wf-panel-title"), uistate.T("workflows.pyfTitle")),
		P(css.Class("wf-panel-desc"), uistate.T("workflows.pyfDesc")),
		If(len(active) > 0, activeBlock),
		If(formVisible, form),
		If(msg.Get() != "", P(css.Class("err"), Attr("role", "alert"), msg.Get())),
		If(success.Get(), P(css.Class("ok"), Attr("role", "status"), uistate.T("workflows.pyfCreated"))),
		IfElse(formVisible,
			Button(css.Class("btn wf-panel-save"), Type("button"), OnClick(save), uistate.T("workflows.pyfSave")),
			Button(css.Class("data-btn wf-panel-save"), Type("button"), Attr("data-testid", "pyf-add-another"), OnClick(onShowForm), uistate.T("wfs.pyfAddAnother")),
		),
	)
}

// anyNodes widens a []ui.Node into []any for variadic element construction.
func anyNodes(ns []ui.Node) []any {
	out := make([]any, len(ns))
	for i, n := range ns {
		out[i] = n
	}
	return out
}

// sweepFormProps passes the refresh callback into the surplus-sweep config card.
type sweepFormProps struct{ Refresh func() }

// sweepForm is the C184 surplus-sweep configuration card. It reads the current
// sweep prefs, lets the user enable/disable the sweep and choose the source
// account, destination account, and buffer floor. Saving writes directly to
// localStorage via uistate.PersistPrefs — the same path used by the appearance
// and date-format prefs. The boot-time RunDueSweeps reads these on next startup.
//
// Its own component so all UseState/UseEvent hooks sit at stable render
// positions, satisfying the framework's no-hooks-in-loops rule.
func sweepForm(_ sweepFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)

	// Seed form state from persisted prefs so the card always reflects the
	// currently saved config, even after a reload.
	p := uistate.LoadPrefs()

	enabled := ui.UseState(p.SweepEnabled)
	fromID := ui.UseState(p.SweepFromAccountID)
	toID := ui.UseState(p.SweepToAccountID)
	bufStr := ui.UseState(func() string {
		if p.SweepBufferMinor > 0 {
			return money.FormatMinor(p.SweepBufferMinor, dec)
		}
		return ""
	}())
	msg := ui.UseState("")
	saved := ui.UseState(false)

	onEnabled := ui.UseEvent(func(e ui.Event) {
		enabled.Set(e.IsChecked())
		msg.Set("")
		saved.Set(false)
	})
	onFrom := ui.UseEvent(func(e ui.Event) {
		fromID.Set(e.GetValue())
		msg.Set("")
		saved.Set(false)
	})
	onTo := ui.UseEvent(func(e ui.Event) {
		toID.Set(e.GetValue())
		msg.Set("")
		saved.Set(false)
	})
	onBuf := ui.UseEvent(func(v string) {
		bufStr.Set(v)
		msg.Set("")
		saved.Set(false)
	})

	save := func() {
		from := fromID.Get()
		to := toID.Get()
		en := enabled.Get()
		if en {
			if from == "" {
				msg.Set(uistate.T("workflows.sweepNeedFrom"))
				return
			}
			if to == "" {
				msg.Set(uistate.T("workflows.sweepNeedTo"))
				return
			}
			if from == to {
				msg.Set(uistate.T("workflows.sweepSameAccount"))
				return
			}
		}
		var buf int64
		raw := strings.TrimSpace(bufStr.Get())
		if raw != "" {
			v, err := money.ParseMinor(raw, dec)
			if err != nil || v < 0 {
				msg.Set(uistate.T("workflows.sweepBadBuffer"))
				return
			}
			buf = v
		}

		cur := uistate.LoadPrefs()
		cur.SweepEnabled = en
		cur.SweepFromAccountID = from
		cur.SweepToAccountID = to
		cur.SweepBufferMinor = buf
		// SweepLastPeriod is intentionally NOT reset here — clearing it would
		// cause an immediate re-sweep on the next boot even if one already
		// ran this month. The user can disable+re-enable to force a re-run.
		uistate.PersistPrefs(cur)
		uistate.SetPrefs(cur)
		msg.Set("")
		saved.Set(true)
	}

	// Account option lists.
	fromNone := Option(Value(""), SelectedIf(fromID.Get() == ""), uistate.T("workflows.pyfChooseAccount"))
	toNone := Option(Value(""), SelectedIf(toID.Get() == ""), uistate.T("workflows.pyfChooseAccount"))
	var fromOpts, toOpts []ui.Node
	fromOpts = append(fromOpts, fromNone)
	toOpts = append(toOpts, toNone)
	for _, ac := range app.Accounts() {
		if ac.Archived {
			continue
		}
		label := ac.Name + " (" + string(ac.Type) + ")"
		fromOpts = append(fromOpts, Option(Value(ac.ID), SelectedIf(fromID.Get() == ac.ID), label))
		toOpts = append(toOpts, Option(Value(ac.ID), SelectedIf(toID.Get() == ac.ID), label))
	}

	bufPlaceholder := uistate.T("workflows.sweepBuffer", base)

	return Div(css.Class("wf-quick-panel"),
		H4(css.Class("wf-panel-title"), uistate.T("workflows.sweepTitle")),
		P(css.Class("wf-panel-desc"), uistate.T("workflows.sweepDesc")),
		P(css.Class("wf-hint"), uistate.T("wfs.bootNote")),
		Div(css.Class("wf-panel-enable"),
			Label(css.Class("checkbox-label"),
				Input(
					Type("checkbox"),
					Attr("aria-label", uistate.T("workflows.sweepEnable")),
					CheckedIf(enabled.Get()),
					OnChange(onEnabled),
				),
				Text(" "+uistate.T("workflows.sweepEnable")),
			),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.sweepFrom")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.sweepFrom")), OnChange(onFrom), fromOpts),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.sweepTo")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.sweepTo")), OnChange(onTo), toOpts),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), bufPlaceholder),
			Input(css.Class("field"), Attr("placeholder", bufPlaceholder), Attr("aria-label", bufPlaceholder),
				Attr("inputmode", "decimal"), Value(bufStr.Get()), OnInput(onBuf)),
		),
		If(msg.Get() != "", P(css.Class("err"), Attr("role", "alert"), msg.Get())),
		If(saved.Get(), P(css.Class("ok"), Attr("role", "status"), uistate.T("workflows.sweepSaved"))),
		Button(css.Class("btn wf-panel-save"), Type("button"), OnClick(save), uistate.T("workflows.sweepSave")),
	)
}

// roundUpFormProps passes the refresh callback into the round-up config card.
type roundUpFormProps struct{ Refresh func() }

// roundUpForm is the C183 monthly round-up savings configuration card. It lets
// the user enable the round-up feature, choose the spending account to round up
// and the savings destination, and set the rounding granularity (nearest $1,
// $5, or $10). Saving writes directly to localStorage via uistate.PersistPrefs.
// The boot-time RunDueRoundUps reads these on next startup.
//
// Its own component so all UseState/UseEvent hooks sit at stable render
// positions, satisfying the framework's no-hooks-in-loops rule.
func roundUpForm(_ roundUpFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}

	// Seed form state from persisted prefs.
	p := uistate.LoadPrefs()

	enabled := ui.UseState(p.RoundUpEnabled)
	fromID := ui.UseState(p.RoundUpFromAccountID)
	toID := ui.UseState(p.RoundUpToAccountID)
	granStr := ui.UseState(func() string {
		switch p.RoundUpGranularityMinor {
		case 500:
			return "500"
		case 1000:
			return "1000"
		default:
			return "100"
		}
	}())
	msg := ui.UseState("")
	saved := ui.UseState(false)

	onEnabled := ui.UseEvent(func(e ui.Event) {
		enabled.Set(e.IsChecked())
		msg.Set("")
		saved.Set(false)
	})
	onFrom := ui.UseEvent(func(e ui.Event) {
		fromID.Set(e.GetValue())
		msg.Set("")
		saved.Set(false)
	})
	onTo := ui.UseEvent(func(e ui.Event) {
		toID.Set(e.GetValue())
		msg.Set("")
		saved.Set(false)
	})
	onGran := ui.UseEvent(func(e ui.Event) {
		granStr.Set(e.GetValue())
		msg.Set("")
		saved.Set(false)
	})

	save := func() {
		from := fromID.Get()
		to := toID.Get()
		en := enabled.Get()
		if en {
			if from == "" {
				msg.Set(uistate.T("workflows.roundUpNeedFrom"))
				return
			}
			if to == "" {
				msg.Set(uistate.T("workflows.roundUpNeedTo"))
				return
			}
			if from == to {
				msg.Set(uistate.T("workflows.roundUpSameAccount"))
				return
			}
		}
		var gran int64 = 100
		switch granStr.Get() {
		case "500":
			gran = 500
		case "1000":
			gran = 1000
		}

		cur := uistate.LoadPrefs()
		cur.RoundUpEnabled = en
		cur.RoundUpFromAccountID = from
		cur.RoundUpToAccountID = to
		cur.RoundUpGranularityMinor = gran
		// RoundUpLastPeriod is intentionally NOT reset here — the user can
		// disable+re-enable to force an immediate re-run if desired.
		uistate.PersistPrefs(cur)
		uistate.SetPrefs(cur)
		msg.Set("")
		saved.Set(true)
	}

	// Account option lists.
	fromNone := Option(Value(""), SelectedIf(fromID.Get() == ""), uistate.T("workflows.pyfChooseAccount"))
	toNone := Option(Value(""), SelectedIf(toID.Get() == ""), uistate.T("workflows.pyfChooseAccount"))
	var fromOpts, toOpts []ui.Node
	fromOpts = append(fromOpts, fromNone)
	toOpts = append(toOpts, toNone)
	for _, ac := range app.Accounts() {
		if ac.Archived {
			continue
		}
		label := ac.Name + " (" + string(ac.Type) + ")"
		fromOpts = append(fromOpts, Option(Value(ac.ID), SelectedIf(fromID.Get() == ac.ID), label))
		toOpts = append(toOpts, Option(Value(ac.ID), SelectedIf(toID.Get() == ac.ID), label))
	}

	return Div(css.Class("wf-quick-panel"),
		H4(css.Class("wf-panel-title"), uistate.T("workflows.roundUpTitle")),
		P(css.Class("wf-panel-desc"), uistate.T("workflows.roundUpDesc")),
		P(css.Class("wf-hint"), uistate.T("wfs.bootNote")),
		Div(css.Class("wf-panel-enable"),
			Label(css.Class("checkbox-label"),
				Input(
					Type("checkbox"),
					Attr("aria-label", uistate.T("workflows.roundUpEnable")),
					CheckedIf(enabled.Get()),
					OnChange(onEnabled),
				),
				Text(" "+uistate.T("workflows.roundUpEnable")),
			),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.roundUpFrom")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.roundUpFrom")), OnChange(onFrom), fromOpts),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.roundUpTo")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.roundUpTo")), OnChange(onTo), toOpts),
		),
		Label(css.Class("fld-field"),
			Span(css.Class("fld-lbl"), uistate.T("workflows.roundUpGran")),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.roundUpGran")), OnChange(onGran),
				Option(Value("100"), SelectedIf(granStr.Get() == "100"), uistate.T("workflows.roundUpGranDollar")),
				Option(Value("500"), SelectedIf(granStr.Get() == "500"), uistate.T("workflows.roundUpGranFive")),
				Option(Value("1000"), SelectedIf(granStr.Get() == "1000"), uistate.T("workflows.roundUpGranTen")),
			),
		),
		If(msg.Get() != "", P(css.Class("err"), Attr("role", "alert"), msg.Get())),
		If(saved.Get(), P(css.Class("ok"), Attr("role", "status"), uistate.T("workflows.roundUpSaved"))),
		Button(css.Class("btn wf-panel-save"), Type("button"), OnClick(save), uistate.T("workflows.roundUpSave")),
	)
}

type addWorkflowFormProps struct{ Refresh func() }

// addWorkflowForm builds a new workflow: name, trigger, optional condition, and a
// list of actions assembled one at a time. A single stable component so its many
// form hooks never run inside a loop.
func addWorkflowForm(props addWorkflowFormProps) ui.Node {
	name := ui.UseState("")
	trigger := ui.UseState(string(workflow.TriggerManual))
	cadence := ui.UseState(string(domain.CadenceMonthly))
	condition := ui.UseState("")
	actions := ui.UseState([]workflow.Action(nil))
	draftKind := ui.UseState(string(workflow.ActionCreateTask))
	draftText := ui.UseState("")
	draftCat := ui.UseState("")
	draftFrom := ui.UseState("")
	draftTo := ui.UseState("")
	draftAmt := ui.UseState("")
	msg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTrigger := ui.UseEvent(func(v string) {
		trigger.Set(v)
		// Keep the drafted action legal for the new trigger: the three
		// transaction-mutating actions only exist on txn-added; transfers are
		// forbidden ON txn-added (loop guard).
		k := workflow.ActionKind(draftKind.Get())
		txnOnly := k == workflow.ActionSetCategory || k == workflow.ActionAddTag || k == workflow.ActionFlagReview
		if (txnOnly && v != string(workflow.TriggerTxnAdded)) ||
			(k == workflow.ActionTransfer && v == string(workflow.TriggerTxnAdded)) {
			draftKind.Set(string(workflow.ActionCreateTask))
		}
	})
	onCadence := ui.UseEvent(func(v string) { cadence.Set(v) })
	onCondition := ui.UseEvent(func(v string) { condition.Set(v) })
	onDraftKind := ui.UseEvent(func(v string) { draftKind.Set(v) })
	onDraftText := ui.UseEvent(func(v string) { draftText.Set(v) })
	onDraftCat := ui.UseEvent(func(e ui.Event) { draftCat.Set(e.GetValue()) })
	onDraftFrom := ui.UseEvent(func(e ui.Event) { draftFrom.Set(e.GetValue()) })
	onDraftTo := ui.UseEvent(func(e ui.Event) { draftTo.Set(e.GetValue()) })
	onDraftAmt := ui.UseEvent(func(v string) { draftAmt.Set(v) })

	base := "USD"
	if app := appstate.Default; app != nil && app.Settings().BaseCurrency != "" {
		base = app.Settings().BaseCurrency
	}
	dec := currency.Decimals(base)

	// buildDraft turns the current action-builder fields into an Action, reporting
	// whether it's complete enough to add (so an empty draft isn't staged, and a
	// half-filled one gives a reason rather than failing silently).
	buildDraft := func() (workflow.Action, bool) {
		a := workflow.Action{Kind: workflow.ActionKind(draftKind.Get())}
		switch a.Kind {
		case workflow.ActionCreateTask:
			a.Title = strings.TrimSpace(draftText.Get())
			return a, a.Title != ""
		case workflow.ActionNotify:
			a.Message = strings.TrimSpace(draftText.Get())
			return a, a.Message != ""
		case workflow.ActionAddTag:
			a.Tag = strings.TrimSpace(draftText.Get())
			return a, a.Tag != ""
		case workflow.ActionSetCategory:
			a.CategoryID = draftCat.Get()
			return a, a.CategoryID != ""
		case workflow.ActionTransfer:
			a.TransferFromAccountID = draftFrom.Get()
			a.TransferToAccountID = draftTo.Get()
			amt, err := money.ParseMinor(strings.TrimSpace(draftAmt.Get()), dec)
			if err != nil || amt <= 0 {
				return a, false
			}
			a.TransferAmount = amt
			return a, a.TransferFromAccountID != "" && a.TransferToAccountID != "" &&
				a.TransferFromAccountID != a.TransferToAccountID
		case workflow.ActionPostRecurring, workflow.ActionFlagBudgetOver:
			return a, true
		default: // applyRules / flagReview need no parameter
			return a, true
		}
	}
	addAction := func() {
		a, ok := buildDraft()
		if !ok {
			msg.Set(uistate.T("workflows.actionNeedsValue"))
			return
		}
		actions.Set(append(append([]workflow.Action(nil), actions.Get()...), a))
		draftText.Set("")
		draftCat.Set("")
		draftFrom.Set("")
		draftTo.Set("")
		draftAmt.Set("")
		msg.Set("")
	}
	// Drop a staged action before saving, so a mistaken one doesn't force starting
	// the whole workflow over (C65).
	removeAction := func(i int) {
		cur := actions.Get()
		if i < 0 || i >= len(cur) {
			return
		}
		next := append([]workflow.Action(nil), cur[:i]...)
		next = append(next, cur[i+1:]...)
		actions.Set(next)
	}
	save := func() {
		app := appstate.Default
		// Fold in a still-pending action the user typed but didn't click "Add
		// action" for, so a filled-but-unstaged action isn't silently lost (C37).
		acts := append([]workflow.Action(nil), actions.Get()...)
		if a, ok := buildDraft(); ok {
			acts = append(acts, a)
		}
		trig := workflow.Trigger{Kind: workflow.TriggerKind(trigger.Get())}
		if trig.Kind == workflow.TriggerScheduled {
			trig.Cadence = domain.RecurringCadence(cadence.Get())
			trig.NextRun = trig.Cadence.Next(time.Now())
		}
		w := workflow.Workflow{
			ID: id.New(), Name: name.Get(), Enabled: true,
			Trigger:   trig,
			Condition: condition.Get(), Actions: acts,
		}
		// Scheduled transfers get a per-period dedupe scope ({period} resolves
		// at each run) so a double-fire within one period moves money once;
		// manual transfers stay unscoped — each deliberate click transfers.
		if trig.Kind == workflow.TriggerScheduled {
			for i := range w.Actions {
				if w.Actions[i].Kind == workflow.ActionTransfer {
					w.Actions[i].DedupeKey = "wf:" + w.ID + ":{period}"
				}
			}
		}
		if errs := workflow.Validate(w); len(errs) > 0 {
			msg.Set(errs[0])
			return
		}
		if err := app.PutWorkflow(w); err != nil {
			msg.Set(err.Error())
			return
		}
		name.Set("")
		condition.Set("")
		actions.Set(nil)
		draftText.Set("")
		draftCat.Set("")
		draftFrom.Set("")
		draftTo.Set("")
		draftAmt.Set("")
		msg.Set("")
		if props.Refresh != nil {
			props.Refresh()
		}
	}

	// The action parameter control depends on the chosen action kind.
	var paramControl ui.Node
	switch workflow.ActionKind(draftKind.Get()) {
	case workflow.ActionSetCategory:
		opts := []ui.Node{Option(Value(""), uistate.T("workflows.chooseCategory"))}
		if appstate.Default != nil {
			for _, c := range appstate.Default.Categories() {
				opts = append(opts, Option(Value(c.ID), SelectedIf(draftCat.Get() == c.ID), c.Name))
			}
		}
		paramControl = Select(css.Class("field"), OnChange(onDraftCat), opts)
	case workflow.ActionTransfer:
		fromOpts := []ui.Node{Option(Value(""), SelectedIf(draftFrom.Get() == ""), uistate.T("workflows.pyfChooseAccount"))}
		toOpts := []ui.Node{Option(Value(""), SelectedIf(draftTo.Get() == ""), uistate.T("workflows.pyfChooseAccount"))}
		if app := appstate.Default; app != nil {
			for _, ac := range app.Accounts() {
				if ac.Archived {
					continue
				}
				label := ac.Name + " (" + string(ac.Type) + ")"
				fromOpts = append(fromOpts, Option(Value(ac.ID), SelectedIf(draftFrom.Get() == ac.ID), label))
				toOpts = append(toOpts, Option(Value(ac.ID), SelectedIf(draftTo.Get() == ac.ID), label))
			}
		}
		amtLabel := uistate.T("wfs.transferAmount", base)
		paramControl = Fragment(
			Label(css.Class("fld-field"),
				Span(css.Class("fld-lbl"), uistate.T("wfs.transferFrom")),
				Select(css.Class("field"), Attr("aria-label", uistate.T("wfs.transferFrom")), OnChange(onDraftFrom), fromOpts)),
			Label(css.Class("fld-field"),
				Span(css.Class("fld-lbl"), uistate.T("wfs.transferTo")),
				Select(css.Class("field"), Attr("aria-label", uistate.T("wfs.transferTo")), OnChange(onDraftTo), toOpts)),
			Label(css.Class("fld-field"),
				Span(css.Class("fld-lbl"), amtLabel),
				Input(css.Class("field"), Attr("placeholder", amtLabel), Attr("aria-label", amtLabel),
					Attr("inputmode", "decimal"), Value(draftAmt.Get()), OnInput(onDraftAmt))),
		)
	case workflow.ActionApplyRules, workflow.ActionFlagReview, workflow.ActionPostRecurring, workflow.ActionFlagBudgetOver:
		paramControl = P(css.Class("muted"), uistate.T("workflows.noParam"))
	default: // createTask / notify / addTag
		paramControl = Input(css.Class("field"), Attr("placeholder", uistate.T("workflows.actionText")),
			Attr("aria-label", uistate.T("workflows.actionText")),
			Value(draftText.Get()), OnInput(onDraftText))
	}

	// Rendered list of staged actions.
	var staged []ui.Node
	for i, a := range actions.Get() {
		staged = append(staged, ui.CreateElement(stagedActionRow, stagedActionRowProps{
			Label: actionLabel(a), Index: i, OnRemove: removeAction,
		}))
	}

	// insertVar appends a workflow condition variable token to the condition
	// input. Each variable gets its own component (condVarButton) so the handler
	// hook lives at a stable position — never called inside a loop (framework
	// gotcha). The insert callback is a plain func passed as a prop.
	insertVar := func(v string) {
		cur := condition.Get()
		if cur != "" {
			condition.Set(cur + " " + v)
		} else {
			condition.Set(v)
		}
	}

	// The footprint: read the draft back as one plain-English sentence so the
	// user can check their automation before saving it.
	when := uistate.T("wfs.whenManual")
	switch workflow.TriggerKind(trigger.Get()) {
	case workflow.TriggerTxnAdded:
		when = uistate.T("wfs.whenTxn")
	case workflow.TriggerScheduled:
		switch domain.RecurringCadence(cadence.Get()) {
		case domain.CadenceWeekly:
			when = uistate.T("wfs.whenWeekly")
		case domain.CadenceQuarterly:
			when = uistate.T("wfs.whenQuarterly")
		case domain.CadenceYearly:
			when = uistate.T("wfs.whenYearly")
		default:
			when = uistate.T("wfs.whenMonthly")
		}
	case workflow.TriggerBudgetExceeded:
		when = uistate.T("wfs.whenBudget")
	case workflow.TriggerGoalReached:
		when = uistate.T("wfs.whenGoal")
	case workflow.TriggerBillDue:
		when = uistate.T("wfs.whenBill")
	}
	footActs := append([]workflow.Action(nil), actions.Get()...)
	if a, ok := buildDraft(); ok {
		footActs = append(footActs, a)
	}
	sentence := when
	rawCond := ""
	if c := strings.TrimSpace(condition.Get()); c != "" {
		// Plain English wherever the condition can be confidently translated;
		// the raw formula stays visible beneath as the auditable form.
		if eng, ok := wfCondEnglish(c); ok {
			sentence += ", " + uistate.T("wfs.ifPart", eng)
			rawCond = c
		} else {
			sentence += ", " + uistate.T("wfs.ifPart", c)
		}
	}
	footKids := []any{css.Class("fld-foot"),
		Span(css.Class("fld-foot-title"), uistate.T("wfs.footTitle")),
	}
	if len(footActs) == 0 {
		footKids = append(footKids, P(css.Class("fld-foot-line"), sentence+" "+uistate.T("wfs.thenNothing")))
	} else {
		footKids = append(footKids, P(css.Class("fld-foot-line"), sentence+", "+uistate.T("wfs.thenPart")))
		for _, a := range footActs {
			footKids = append(footKids, P(css.Class("fld-foot-line"), "• "+actionLabel(a)))
		}
	}
	if rawCond != "" {
		footKids = append(footKids, Span(css.Class("wf-foot-raw"), rawCond))
	}

	fld := func(lbl string, control ui.Node) ui.Node {
		return Label(css.Class("fld-field"), Span(css.Class("fld-lbl"), lbl), control)
	}

	// Live sanity-check of the condition draft: a typo, an unknown variable,
	// or a txn_* variable on a non-transaction trigger warns HERE instead of
	// surfacing later as a failed run. Placeholder txn values join the check
	// surface only when the trigger actually provides them.
	condWarn := ""
	if c := strings.TrimSpace(condition.Get()); c != "" && appstate.Default != nil {
		checkCtx := workflow.Context{Vars: map[string]float64{}, Strs: map[string]string{}}
		for k, v := range vbVariableSurface() {
			checkCtx.Vars[k] = v
		}
		if trigger.Get() == string(workflow.TriggerTxnAdded) {
			checkCtx.Vars["txn_amount"], checkCtx.Vars["txn_abs"] = 0, 0
			for _, k := range []string{"txn_payee", "txn_desc", "txn_category", "txn_account", "txn_tags"} {
				checkCtx.Strs[k] = ""
			}
			for _, def := range appstate.Default.CustomFieldDefsFor("transaction") {
				switch def.Type {
				case customfields.TypeNumber, customfields.TypeBool:
					checkCtx.Vars["cf_txn_"+def.Key] = 0
				default:
					checkCtx.Strs["cf_txn_"+def.Key] = ""
				}
			}
		}
		if _, err := workflow.Eval(c, checkCtx); err != nil {
			condWarn = err.Error()
		}
	}

	return Div(css.Class("wf-composer"), Attr("data-testid", "wf-composer"),
		H3(css.Class("wf-comp-title"), uistate.T("workflows.create")),
		P(css.Class("wf-comp-lede"), uistate.T("wfs.compLede")),
		fld(uistate.T("workflows.name"),
			Input(css.Class("field"), Attr("placeholder", uistate.T("workflows.name")), Attr("aria-label", uistate.T("workflows.name")), Value(name.Get()), OnInput(onName))),
		fld(uistate.T("workflows.triggerLabel"),
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.triggerLabel")), OnChange(onTrigger),
				Option(Value(string(workflow.TriggerManual)), SelectedIf(trigger.Get() == string(workflow.TriggerManual)), uistate.T("workflows.triggerManual")),
				Option(Value(string(workflow.TriggerTxnAdded)), SelectedIf(trigger.Get() == string(workflow.TriggerTxnAdded)), uistate.T("workflows.triggerTxn")),
				Option(Value(string(workflow.TriggerScheduled)), SelectedIf(trigger.Get() == string(workflow.TriggerScheduled)), uistate.T("workflows.triggerScheduled")),
				Option(Value(string(workflow.TriggerBudgetExceeded)), SelectedIf(trigger.Get() == string(workflow.TriggerBudgetExceeded)), uistate.T("workflows.triggerBudgetExceeded")),
				Option(Value(string(workflow.TriggerGoalReached)), SelectedIf(trigger.Get() == string(workflow.TriggerGoalReached)), uistate.T("workflows.triggerGoalReached")),
				Option(Value(string(workflow.TriggerBillDue)), SelectedIf(trigger.Get() == string(workflow.TriggerBillDue)), uistate.T("workflows.triggerBillDue")),
			)),
		If(trigger.Get() == string(workflow.TriggerScheduled),
			fld(uistate.T("wfs.cadence"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("wfs.cadence")), OnChange(onCadence),
					Option(Value(string(domain.CadenceWeekly)), SelectedIf(cadence.Get() == string(domain.CadenceWeekly)), uistate.T("workflows.cadenceWeekly")),
					Option(Value(string(domain.CadenceMonthly)), SelectedIf(cadence.Get() == string(domain.CadenceMonthly)), uistate.T("workflows.cadenceMonthly")),
					Option(Value(string(domain.CadenceQuarterly)), SelectedIf(cadence.Get() == string(domain.CadenceQuarterly)), uistate.T("workflows.cadenceQuarterly")),
					Option(Value(string(domain.CadenceYearly)), SelectedIf(cadence.Get() == string(domain.CadenceYearly)), uistate.T("workflows.cadenceYearly")),
				)),
		),
		fld(uistate.T("workflows.conditionLabel"),
			Input(css.Class("field"), Attr("placeholder", uistate.T("wfs.condPlaceholder")), Attr("aria-label", uistate.T("workflows.conditionLabel")), Value(condition.Get()), OnInput(onCondition))),
		If(condWarn != "", P(css.Class("wf-cond-warn"), Attr("role", "status"), Attr("data-testid", "wf-cond-warn"), uistate.T("wfs.condWarn", condWarn))),
		// Inline variable reference for the condition formula (C65): the four
		// txn_* chips (each its own component so its OnClick hook never runs
		// inside a variable-length loop), plus an insert-any-variable dropdown
		// over the FULL engine surface the runners evaluate — custom fields,
		// molecules, budget/goal figures included.
		Div(css.Class("wf-cond-help"),
			P(css.Class("wf-hint"), uistate.T("workflows.conditionHint")),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap15),
				ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_abs", Desc: uistate.T("workflows.varTxnAbs"), OnInsert: insertVar}),
				ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_amount", Desc: uistate.T("workflows.varTxnAmount"), OnInsert: insertVar}),
				ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_payee", Desc: uistate.T("workflows.varTxnPayee"), OnInsert: insertVar}),
				ui.CreateElement(condVarButton, condVarButtonProps{Token: "txn_category", Desc: uistate.T("workflows.varTxnCategory"), OnInsert: insertVar}),
			),
			ui.CreateElement(wfVarInsert, wfVarInsertProps{OnInsert: insertVar}),
			P(css.Class("wf-hint"), uistate.T("workflows.conditionExamples")),
		),
		Div(css.Class("wf-actions-head"), Span(css.Class("fld-lbl"), uistate.T("wfs.actionsHead"))),
		fld(uistate.T("workflows.actionTypeLabel"),
			// The option set follows the trigger: setCategory/addTag/flagReview
			// mutate the TRIGGERING transaction and only exist on txn-added;
			// transfer is forbidden on txn-added (loop guard) — offering an
			// illegal combination just to reject it at save is worse.
			Select(css.Class("field"), Attr("aria-label", uistate.T("workflows.actionTypeLabel")), OnChange(onDraftKind),
				Option(Value(string(workflow.ActionCreateTask)), SelectedIf(draftKind.Get() == string(workflow.ActionCreateTask)), uistate.T("workflows.actCreateTask")),
				If(trigger.Get() == string(workflow.TriggerTxnAdded), Fragment(
					Option(Value(string(workflow.ActionSetCategory)), SelectedIf(draftKind.Get() == string(workflow.ActionSetCategory)), uistate.T("workflows.actSetCategory")),
					Option(Value(string(workflow.ActionAddTag)), SelectedIf(draftKind.Get() == string(workflow.ActionAddTag)), uistate.T("workflows.actAddTag")),
					Option(Value(string(workflow.ActionFlagReview)), SelectedIf(draftKind.Get() == string(workflow.ActionFlagReview)), uistate.T("workflows.actFlagReview")),
				)),
				Option(Value(string(workflow.ActionApplyRules)), SelectedIf(draftKind.Get() == string(workflow.ActionApplyRules)), uistate.T("workflows.actApplyRules")),
				Option(Value(string(workflow.ActionNotify)), SelectedIf(draftKind.Get() == string(workflow.ActionNotify)), uistate.T("workflows.actNotify")),
				Option(Value(string(workflow.ActionPostRecurring)), SelectedIf(draftKind.Get() == string(workflow.ActionPostRecurring)), uistate.T("workflows.actPostRecurring")),
				Option(Value(string(workflow.ActionFlagBudgetOver)), SelectedIf(draftKind.Get() == string(workflow.ActionFlagBudgetOver)), uistate.T("workflows.actFlagBudgetOver")),
				If(trigger.Get() != string(workflow.TriggerTxnAdded),
					Option(Value(string(workflow.ActionTransfer)), SelectedIf(draftKind.Get() == string(workflow.ActionTransfer)), uistate.T("wfs.actTransfer"))),
			)),
		Div(css.Class("wf-param"), paramControl),
		P(css.Class("wf-hint"), uistate.T("wfs.tmplHint")),
		Button(css.Class("btn btn-sm wf-addaction"), Type("button"), OnClick(addAction), uistate.T("workflows.addAction")),
		If(len(staged) > 0, Div(css.Class("wf-staged"), staged)),
		Div(footKids...),
		If(msg.Get() != "", P(css.Class("err"), Attr("role", "alert"), msg.Get())),
		Button(css.Class("btn btn-primary wf-save"), Type("button"), OnClick(save), uistate.T("workflows.save")),
	)
}

type workflowRowProps struct {
	Workflow workflow.Workflow
	Refresh  func()
}

// workflowRow is one automation's ledger row: a status dot, the name, its
// trigger · condition · action summary, dry-run-first controls, and a ⋯ menu
// (enable/disable, edit, diagram, delete behind a two-step inline confirm).
// The last run's planned/applied effects show beneath. Its own component so
// the action hooks and the result state stay stable across the list.
func workflowRow(props workflowRowProps) ui.Node {
	w := props.Workflow
	last := ui.UseState((*workflow.Run)(nil))
	lastErr := ui.UseState("")
	showDiagram := ui.UseState(false)
	editing := ui.UseState(false)
	confirming := ui.UseState(false)
	editName := ui.UseState(w.Name)
	editCond := ui.UseState(w.Condition)
	onEditName := ui.UseEvent(func(v string) { editName.Set(v) })
	onEditCond := ui.UseEvent(func(v string) { editCond.Set(v) })
	ask := ui.UseEvent(Prevent(func() {
		confirming.Set(true)
		fldFocusSoon("#wf-keep-" + w.ID)
	}))
	keep := ui.UseEvent(Prevent(func() {
		confirming.Set(false)
		fldFocusSoon("#wf-menu-" + w.ID + " button")
	}))
	startEdit := ui.UseEvent(Prevent(func() {
		editName.Set(w.Name)
		editCond.Set(w.Condition)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		n := strings.TrimSpace(editName.Get())
		if n == "" {
			return
		}
		w2 := w
		w2.Name = n
		w2.Condition = strings.TrimSpace(editCond.Get())
		if err := app.PutWorkflow(w2); err == nil {
			editing.Set(false)
			if props.Refresh != nil {
				props.Refresh()
			}
		}
	}))

	run := func(dry bool) {
		app := appstate.Default
		var r workflow.Run
		var err error
		if w.Trigger.Kind == workflow.TriggerTxnAdded {
			// A txn-added workflow only means something WITH a transaction:
			// dry runs preview against the most recent one (so txn_* and
			// cf_txn_* conditions evaluate truthfully). There is no manual
			// LIVE run — the button is hidden, and this guard keeps a future
			// caller from accidentally mutating the user's latest transaction.
			if !dry {
				lastErr.Set(uistate.T("wfs.dryNeedsTxn"))
				last.Set(nil)
				return
			}
			t, ok := latestTransaction(app)
			if !ok {
				lastErr.Set(uistate.T("wfs.dryNeedsTxn"))
				last.Set(nil)
				return
			}
			r, err = app.RunWorkflowOn(w, t, true)
		} else {
			r, err = app.RunWorkflow(w, dry)
		}
		if err != nil {
			// A broken condition (typo, unknown variable, txn_* var with no
			// transaction in scope) is NOT "didn't match" — it gets its own
			// visible state so the user can tell logic-false from broken.
			lastErr.Set(err.Error())
			last.Set(nil)
			return
		}
		lastErr.Set("")
		rr := r
		last.Set(&rr)
		if !dry && props.Refresh != nil {
			props.Refresh()
		}
	}
	toggle := func() {
		app := appstate.Default
		w2 := w
		w2.Enabled = !w2.Enabled
		if err := app.PutWorkflow(w2); err == nil && props.Refresh != nil {
			props.Refresh()
		}
	}
	del := func() {
		app := appstate.Default
		if err := app.DeleteWorkflow(w.ID); err == nil && props.Refresh != nil {
			props.Refresh()
		}
	}

	if editing.Get() {
		return Div(css.Class("row-edit"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("workflows.name"),
					Input(css.Class("field"), Attr("aria-label", uistate.T("workflows.name")), Value(editName.Get()), OnInput(onEditName))),
				labeledField(uistate.T("workflows.conditionLabel"),
					Input(css.Class("field"), Attr("aria-label", uistate.T("workflows.conditionLabel")), Placeholder(uistate.T("workflows.condition")), Value(editCond.Get()), OnInput(onEditCond))),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	enableLabel := uistate.T("workflows.enable")
	if w.Enabled {
		enableLabel = uistate.T("workflows.disable")
	}
	diagramLabel := uistate.T("wfs.diagramShow")
	if showDiagram.Get() {
		diagramLabel = uistate.T("wfs.diagramHide")
	}

	var result ui.Node = Fragment()
	if e := lastErr.Get(); e != "" {
		result = P(css.Class("wf-result-err"), Attr("role", "alert"), uistate.T("wfs.runErr", e))
	} else if r := last.Get(); r != nil {
		if !r.Matched && !r.DryRun {
			result = P(css.Class("wf-result-quiet"), uistate.T("workflows.noMatch"))
		} else if !r.Matched && r.DryRun {
			result = P(css.Class("wf-result-quiet"), uistate.T("workflows.dryNoMatch"))
		} else {
			var lines []ui.Node
			allSkipped := len(r.Effects) > 0
			for _, e := range r.Effects {
				lines = append(lines, Div(css.Class("wf-result-line"), "• "+e.Summary))
				if !strings.Contains(e.Summary, "skipped: no transaction") {
					allSkipped = false
				}
			}
			head := uistate.T("workflows.applied")
			if r.DryRun {
				head = uistate.T("workflows.wouldDo")
			}
			if allSkipped {
				// Every effect skipped for lack of a transaction — "DONE" would
				// oversell a run that changed nothing.
				head = uistate.T("wfs.nothingHead")
			}
			var note ui.Node = Fragment()
			if r.DryRun && w.Trigger.Kind == workflow.TriggerTxnAdded {
				note = Div(css.Class("wf-result-line", tw.TextDim), uistate.T("wfs.dryPreviewTxn"))
			}
			result = Div(css.Class("wf-result"), Span(css.Class("wf-result-head"), head), note, Div(lines))
		}
	}

	rowCls := "wf-row"
	if !w.Enabled {
		rowCls += " is-off"
	}

	dot := Span(css.Class("wf-dot"), Attr("aria-hidden", "true"))
	condPart := Fragment()
	if w.Condition != "" {
		// Plain English where translatable (raw formula kept as the hover title);
		// otherwise the raw expression in mono.
		if eng, ok := wfCondEnglish(w.Condition); ok {
			condPart = Fragment(Span(css.Class("wf-meta-sep"), " · "),
				Span(Attr("title", w.Condition), uistate.T("wfs.ifPart", eng)))
		} else {
			condPart = Fragment(Span(css.Class("wf-meta-sep"), " · "), Span(css.Class("wf-cond"), "if "+w.Condition))
		}
	}

	return Div(ClassStr(rowCls),
		Div(css.Class("wf-row-head"),
			dot,
			Div(css.Class("wf-row-main"),
				Div(css.Class("wf-row-top"),
					Span(css.Class("wf-name"), w.Name),
					If(!w.Enabled, Span(css.Class("wman-hidden-tag"), uistate.T("wfs.disabledTag"))),
				),
				Div(css.Class("wf-meta"),
					Span(triggerLabel(w.Trigger.Kind)),
					condPart,
					Span(css.Class("wf-meta-sep"), " · "),
					Span(actionsLabel(len(w.Actions))),
				),
			),
			Div(css.Class("wf-row-actions"),
				// Dry run first — simulation is the safe default instinct (G19).
				Button(css.Class("data-btn wf-dry"), Type("button"), OnClick(func() { run(true) }), uistate.T("workflows.dryRun")),
				// Run now only while the workflow is ON (off means quiet) and
				// only when a manual run can DO something — a txn-added workflow
				// has no transaction in a manual run, so it gets Dry run (which
				// previews against the latest transaction) but no live button.
				If(w.Enabled && w.Trigger.Kind != workflow.TriggerTxnAdded,
					Button(css.Class("data-btn"), Type("button"), OnClick(func() { run(false) }), uistate.T("workflows.runNow"))),
				If(!confirming.Get(),
					uiw.KebabMenu(uiw.KebabMenuProps{
						ID:           "wf-menu-" + w.ID,
						ToggleTestID: "wf-menu-btn-" + w.ID,
						Items: []ui.Node{
							Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(toggle), enableLabel),
							Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(startEdit), uistate.T("action.edit")),
							Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(func() { showDiagram.Set(!showDiagram.Get()) }), diagramLabel),
							Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "wf-delete-btn-"+w.ID), OnClick(ask), uistate.T("action.delete")),
						},
					})),
			),
		),
		If(confirming.Get(), Div(css.Class("fld-confirm"), Attr("role", "alert"),
			Span(css.Class("fld-confirm-msg"), uistate.T("wfs.deleteWarn")),
			Button(css.Class("fld-confirm-del"), Type("button"), OnClick(del), uistate.T("wfs.deleteYes")),
			Button(css.Class("fld-confirm-keep"), Type("button"), Attr("id", "wf-keep-"+w.ID), OnClick(keep), uistate.T("fld.deleteNo")),
		)),
		result,
		// A Mermaid flowchart of this workflow: trigger → condition → actions (C70),
		// collapsed behind the ⋯ menu (GI3: four open diagrams once buried the page).
		If(showDiagram.Get(),
			uiw.Mermaid(uiw.MermaidProps{
				Source: mermaid.FromWorkflow(w),
				Class:  tw.Fold(tw.Mt2),
				Label:  uistate.T("workflows.flowchartOf", w.Name),
			}),
		),
	)
}

// workflowHistoryProps carries the page's refresh revision — its only purpose
// is to make the props differ across refreshes so the memoized component
// re-reads the run list (empty props would be reference-equal forever).
type workflowHistoryProps struct{ Rev int }

// workflowHistory shows the most recent applied runs (newest first).
func workflowHistory(_ workflowHistoryProps) ui.Node {
	app := appstate.Default
	runs := app.WorkflowRuns()
	if len(runs) == 0 {
		return Fragment()
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].At > runs[j].At })
	names := map[string]string{}
	for _, w := range app.Workflows() {
		names[w.ID] = w.Name
	}
	var rows []ui.Node
	for i, r := range runs {
		if i >= 12 {
			break
		}
		name := names[r.WorkflowID]
		if name == "" {
			name = uistate.T("workflows.deleted")
		}
		// Human date (per the user's date-format preference), not raw RFC3339.
		// A stamp of exactly midnight UTC is date-only semantics (seed data) and
		// renders as its UTC date with no time; a real run renders as the LOCAL
		// date + time, so same-day runs stay distinguishable and the date and
		// clock never come from different zones.
		when := r.At
		if t, err := time.Parse(time.RFC3339, r.At); err == nil {
			if utc := t.UTC(); utc.Hour() == 0 && utc.Minute() == 0 && utc.Second() == 0 {
				when = uistate.LoadPrefs().FormatDate(utc)
			} else {
				lt := t.Local()
				when = uistate.LoadPrefs().FormatDate(lt) + " " + lt.Format("3:04 PM")
			}
		}
		effWord := uistate.T("workflows.effectsWord")
		if len(r.Effects) == 1 {
			effWord = uistate.T("wfs.effectWord")
		}
		rows = append(rows, Div(css.Class("wf-hist-row"),
			Span(css.Class("wf-hist-name"), name),
			Span(css.Class("wf-hist-meta"), when+" · "+strconv.Itoa(len(r.Effects))+" "+effWord),
		))
	}
	return Div(css.Class("wf-history"),
		Div(css.Class("wf-sec-head"), H3(css.Class("wf-sec-title"), uistate.T("workflows.history"))),
		Div(css.Class("wf-hist-rows"), rows),
	)
}

// --- label helpers ---

// wfCondEnglish renders a supported single-clause condition formula as plain
// English ("the amount is over 200", `the payee contains "uber"`) so the
// composer footprint and registry rows don't ask users to decode code. It is
// deliberately conservative: compound formulas (&&/||), unknown variables, and
// anything ambiguous return ok=false and the caller falls back to the raw
// expression.
func wfCondEnglish(cond string) (string, bool) {
	c := strings.TrimSpace(cond)
	if c == "" || strings.Contains(c, "&&") || strings.Contains(c, "||") {
		return "", false
	}
	subject := map[string]string{
		"txn_abs":      uistate.T("wfs.subjAbs"),
		"txn_amount":   uistate.T("wfs.subjAmount"),
		"txn_payee":    uistate.T("wfs.subjPayee"),
		"txn_category": uistate.T("wfs.subjCategory"),
	}
	// subj resolves the English subject for a variable: the curated txn_*
	// phrasings first, then any other engine identifier as its spaced name
	// ("safe_to_spend" → "safe to spend") — the full surface is fair game now.
	subj := func(v string) (string, bool) {
		if s, ok := subject[v]; ok {
			return s, true
		}
		if v == "" || (v[0] >= '0' && v[0] <= '9') {
			return "", false
		}
		for _, r := range v {
			if r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return "", false
			}
		}
		return strings.ReplaceAll(v, "_", " "), true
	}
	if strings.HasPrefix(c, "contains(") && strings.HasSuffix(c, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(c, "contains("), ")")
		parts := strings.SplitN(inner, ",", 2)
		if len(parts) == 2 {
			s, ok := subj(strings.TrimSpace(parts[0]))
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if ok && val != "" && !strings.ContainsAny(val, `"'(),`) {
				return uistate.T("wfs.condContains", s, val), true
			}
		}
		return "", false
	}
	for _, op := range []string{">=", "<=", "==", "!=", ">", "<"} {
		i := strings.Index(c, op)
		if i <= 0 {
			continue
		}
		v := strings.TrimSpace(c[:i])
		rhs := strings.TrimSpace(c[i+len(op):])
		s, ok := subj(v)
		if !ok || rhs == "" || strings.ContainsAny(rhs, "<>=&|()") {
			return "", false
		}
		if v == "txn_amount" && rhs == "0" {
			switch op {
			case "<":
				return uistate.T("wfs.condMoneyOut"), true
			case ">":
				return uistate.T("wfs.condMoneyIn"), true
			}
		}
		rhs = strings.Trim(rhs, `"'`)
		switch op {
		case ">":
			return uistate.T("wfs.condOver", s, rhs), true
		case ">=":
			return uistate.T("wfs.condAtLeast", s, rhs), true
		case "<":
			return uistate.T("wfs.condUnder", s, rhs), true
		case "<=":
			return uistate.T("wfs.condAtMost", s, rhs), true
		case "==":
			return uistate.T("wfs.condIs", s, rhs), true
		default: // !=
			return uistate.T("wfs.condIsNot", s, rhs), true
		}
	}
	return "", false
}

func actionLabel(a workflow.Action) string {
	switch a.Kind {
	case workflow.ActionCreateTask:
		return uistate.T("workflows.actCreateTask") + ": " + a.Title
	case workflow.ActionApplyRules:
		return uistate.T("workflows.actApplyRules")
	case workflow.ActionNotify:
		return uistate.T("workflows.actNotify") + ": " + a.Message
	case workflow.ActionSetCategory:
		return uistate.T("workflows.actSetCategory")
	case workflow.ActionAddTag:
		return uistate.T("workflows.actAddTag") + ": " + a.Tag
	case workflow.ActionFlagReview:
		return uistate.T("workflows.actFlagReview")
	case workflow.ActionPostRecurring:
		return uistate.T("workflows.actPostRecurring")
	case workflow.ActionFlagBudgetOver:
		return uistate.T("workflows.actFlagBudgetOver")
	case workflow.ActionTransfer:
		base := "USD"
		name := func(id string) string { return id }
		if app := appstate.Default; app != nil {
			if app.Settings().BaseCurrency != "" {
				base = app.Settings().BaseCurrency
			}
			name = func(id string) string {
				for _, ac := range app.Accounts() {
					if ac.ID == id {
						return ac.Name
					}
				}
				return id
			}
		}
		return uistate.T("wfs.actTransfer") + ": " +
			money.FormatMinor(a.TransferAmount, currency.Decimals(base)) + " " + base + " · " +
			name(a.TransferFromAccountID) + " → " + name(a.TransferToAccountID)
	default:
		return string(a.Kind)
	}
}

// actionsLabel renders the action count with correct singular/plural wording
// ("1 action" vs "2 actions") instead of always-plural "N actions" (C54).
func actionsLabel(n int) string {
	word := uistate.T("workflows.actionsWord")
	if n == 1 {
		word = uistate.T("workflows.actionWord")
	}
	return strconv.Itoa(n) + " " + word
}

func triggerLabel(k workflow.TriggerKind) string {
	switch k {
	case workflow.TriggerTxnAdded:
		return uistate.T("workflows.triggerTxn")
	case workflow.TriggerScheduled:
		return uistate.T("workflows.triggerScheduled")
	case workflow.TriggerBudgetExceeded:
		return uistate.T("workflows.triggerBudgetExceeded")
	case workflow.TriggerGoalReached:
		return uistate.T("workflows.triggerGoalReached")
	case workflow.TriggerBillDue:
		return uistate.T("workflows.triggerBillDue")
	default:
		return uistate.T("workflows.triggerManual")
	}
}

type stagedActionRowProps struct {
	Label    string
	Index    int
	OnRemove func(int)
}

// stagedActionRow renders one staged (not-yet-saved) workflow action with a remove
// button, so a mistaken action can be dropped before saving (C65). It is its own
// component so the remove button's OnClick hook sits at a stable render position —
// the staged list is variable-length (the framework loop-hook gotcha).
func stagedActionRow(props stagedActionRowProps) ui.Node {
	return Div(css.Class("row"),
		Span(css.Class("row-desc"), props.Label),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("workflows.removeAction")), Title(uistate.T("workflows.removeAction")),
			OnClick(func() { props.OnRemove(props.Index) }), "✕"),
	)
}

// latestTransaction returns the most recent non-transfer transaction — the
// context a txn-added workflow's Dry run previews against, so txn_* and
// cf_txn_* conditions evaluate against something real instead of erroring.
func latestTransaction(app *appstate.App) (domain.Transaction, bool) {
	var best domain.Transaction
	found := false
	for _, t := range app.Transactions() {
		if t.IsTransfer() {
			continue
		}
		if !found || t.Date.After(best.Date) {
			best, found = t, true
		}
	}
	return best, found
}

type wfVarInsertProps struct{ OnInsert func(string) }

// wfVarInsert offers every workflow-condition variable — the exact surface the
// trigger runners evaluate against, custom fields and molecules included — as
// an insert-on-choose dropdown beside the txn_* chips, so conditions over any
// engine figure are discoverable rather than memorized. Its own component for
// an isolated change hook; the placeholder re-selects after every insert.
func wfVarInsert(p wfVarInsertProps) ui.Node {
	on := ui.UseEvent(func(e ui.Event) {
		if v := e.GetValue(); v != "" && p.OnInsert != nil {
			p.OnInsert(v)
		}
	})
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	// General figures (atoms, molecules, custom values) lead; the long tail of
	// per-entity variables sits after a separator so income / safe_to_spend /
	// cf_* aren't buried under 40 budget_* rows.
	var general, perEntity []string
	for _, n := range app.WorkflowVariableNames() {
		switch {
		case strings.HasPrefix(n, "budget_"), strings.HasPrefix(n, "goal_"),
			strings.HasPrefix(n, "account_"), strings.HasPrefix(n, "debt_"),
			strings.HasPrefix(n, "plan_"), strings.HasPrefix(n, "pool_"):
			perEntity = append(perEntity, n)
		default:
			general = append(general, n)
		}
	}
	opts := []ui.Node{Option(Value(""), SelectedIf(true), uistate.T("wfs.insertVar"))}
	for _, n := range general {
		opts = append(opts, Option(Value(n), strings.ReplaceAll(n, "_", " ")))
	}
	if len(perEntity) > 0 {
		opts = append(opts, Option(Value(""), Attr("disabled", "disabled"), uistate.T("wfs.perEntitySep")))
		for _, n := range perEntity {
			opts = append(opts, Option(Value(n), strings.ReplaceAll(n, "_", " ")))
		}
	}
	return Select(css.Class("field wf-varselect"), Attr("aria-label", uistate.T("wfs.insertVar")),
		Attr("data-testid", "wf-var-insert"), OnChange(on), opts)
}

type condVarButtonProps struct {
	Token    string       // variable name, e.g. "txn_abs"
	Desc     string       // short description shown as a tooltip
	OnInsert func(string) // called with Token when the user clicks
}

// condVarButton renders one workflow-condition variable as a clickable pill that
// appends the variable name to the condition input (C65). It is its own component
// (not rendered inside a loop) so its OnClick hook always runs at a stable render
// position — satisfying the framework's no-hooks-in-loops rule.
func condVarButton(props condVarButtonProps) ui.Node {
	ins := ui.UseEvent(Prevent(func() { props.OnInsert(props.Token) }))
	return Button(css.Class("data-btn"), Type("button"),
		Attr("data-testid", "cond-var-"+props.Token),
		Title(props.Desc),
		Attr("aria-label", uistate.T("workflows.insertCondVar", props.Token)),
		OnClick(ins), props.Token)
}
