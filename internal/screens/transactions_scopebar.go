// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnscope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// txnScopeLabel describes the ledger's active date filter.
func txnScopeLabel(s txnscope.Scope) string {
	switch s.Kind {
	case txnscope.All:
		return uistate.T("txnperiod.allLabel")
	case txnscope.Month:
		return monthLabel(s.Anchor)
	case txnscope.Day:
		return uistate.T("txnperiod.oneDay", uistate.LoadPrefs().FormatDate(s.From))
	default:
		from, to := "…", "…"
		if !s.From.IsZero() {
			from = uistate.LoadPrefs().FormatDate(s.From)
		}
		if !s.To.IsZero() {
			to = uistate.LoadPrefs().FormatDate(s.To)
		}
		return uistate.T("txnperiod.customRange", from, to)
	}
}

// txnDateScope renders the ledger period controls independently so the
// Transactions toolbar can place them beside Search without moving the view switch.
func txnDateScope(_ struct{}) ui.Node {
	filterAtom := uistate.UseTxFilter()
	f := filterAtom.Get()
	now := time.Now()
	scope := txnscope.Of(f.From, f.To, now)

	setRange := func(from, to, preset string) {
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) {
			x.From, x.To, x.PeriodPreset = from, to, preset
		})
	}
	setScopeStep := func(direction int) {
		from, to, preset := txnScopeStepRange(scope, direction)
		setRange(from, to, preset)
	}
	preset := txnScopePreset(f, now)
	customOpen := ui.UseState(false)
	draftFrom := ui.UseState(f.From)
	draftTo := ui.UseState(f.To)
	prev := ui.UseEvent(Prevent(func() {
		customOpen.Set(false)
		setScopeStep(-1)
	}))
	next := ui.UseEvent(Prevent(func() {
		customOpen.Set(false)
		setScopeStep(1)
	}))
	clearDates := ui.UseEvent(Prevent(func() {
		customOpen.Set(false)
		setRange("", "", txnfilter.PeriodAllDates)
	}))
	onPreset := ui.UseEvent(func(value string) {
		if value == "custom" {
			draftFrom.Set(f.From)
			draftTo.Set(f.To)
			customOpen.Set(true)
			return
		}
		customOpen.Set(false)
		if from, to, key, ok := txnScopePresetRange(value, now); ok {
			setRange(from, to, key)
		}
	})
	editDates := ui.UseEvent(Prevent(func() {
		draftFrom.Set(f.From)
		draftTo.Set(f.To)
		customOpen.Set(!customOpen.Get())
	}))
	onFrom := ui.UseEvent(func(value string) {
		draftFrom.Set(value)
	})
	onTo := ui.UseEvent(func(value string) {
		draftTo.Set(value)
	})
	applyDates := ui.UseEvent(Prevent(func() {
		from, to, key := txnScopeCustomRange(draftFrom.Get(), draftTo.Get())
		if from != "" && to != "" && from > to {
			return
		}
		setRange(from, to, key)
		customOpen.Set(false)
	}))
	cancelDates := ui.UseEvent(Prevent(func() { customOpen.Set(false) }))
	// Keep the editor open while focus moves among its controls (including the
	// browser's native date picker), but dismiss it when the user leaves it.
	ui.UseEffect(func() func() {
		if !customOpen.Get() {
			return nil
		}
		doc := js.Global().Get("document")
		if !doc.Truthy() || doc.Get("addEventListener").Type() != js.TypeFunction {
			return nil // The WASM test renderer has no document.
		}
		inside := func(target js.Value) bool {
			if !target.Truthy() {
				return false
			}
			control := doc.Call("getElementById", "txn-period-control")
			return control.Truthy() && control.Call("contains", target).Bool()
		}
		onPointerDown := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && !inside(args[0].Get("target")) {
				customOpen.Set(false)
			}
			return nil
		})
		onFocusOut := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 || !inside(args[0].Get("target")) {
				return nil
			}
			next := args[0].Get("relatedTarget")
			if next.Truthy() && !inside(next) {
				customOpen.Set(false)
			}
			return nil
		})
		onKeyDown := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Get("key").String() == "Escape" && inside(args[0].Get("target")) {
				args[0].Call("preventDefault")
				customOpen.Set(false)
			}
			return nil
		})
		doc.Call("addEventListener", "pointerdown", onPointerDown)
		doc.Call("addEventListener", "focusout", onFocusOut)
		doc.Call("addEventListener", "keydown", onKeyDown)
		return func() {
			doc.Call("removeEventListener", "pointerdown", onPointerDown)
			doc.Call("removeEventListener", "focusout", onFocusOut)
			doc.Call("removeEventListener", "keydown", onKeyDown)
			onPointerDown.Release()
			onFocusOut.Release()
			onKeyDown.Release()
		}
	}, customOpen.Get())

	selectedPreset := preset
	if customOpen.Get() {
		selectedPreset = "custom"
	}
	custom := customOpen.Get()
	validRange := draftFrom.Get() == "" || draftTo.Get() == "" || draftFrom.Get() <= draftTo.Get()
	customLabel := uistate.T("txnperiod.customRangeChoice")
	if scope.Kind == txnscope.Day {
		customLabel = txnScopeLabel(scope)
	}
	controlClass := "txn-period-control"
	if selectedPreset == "custom" {
		controlClass += " has-edit"
	}
	prevDisabled := scope.Kind == txnscope.All
	nextDisabled := !scope.CanStepForward(now)
	hasDateFilter := f.From != "" || f.To != ""

	return Div(css.Class("txn-scopebar"), Attr("data-testid", "txn-scopebar"),
		Attr("role", "group"), Attr("aria-label", uistate.T("txnperiod.scopeAria", txnScopeLabel(scope))),
		Button(css.Class("txn-scope-step"), Type("button"),
			Attr("data-testid", "txn-scope-prev"),
			Disabled(prevDisabled),
			Attr("aria-label", uistate.T("txnperiod.prev")), Title(uistate.T("txnperiod.prev")),
			OnClick(prev), uiw.Icon(icon.ChevronLeft, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Div(ClassStr(controlClass), Attr("id", "txn-period-control"),
			Select(css.Class("txn-period-select"), Attr("data-testid", "txn-period-select"),
				Attr("aria-label", uistate.T("txnperiod.periodLabel")), OnChange(onPreset),
				Option(Value("this-month"), SelectedIf(selectedPreset == "this-month"), uistate.T("txnperiod.thisMonth")),
				Option(Value("last-month"), SelectedIf(selectedPreset == "last-month"), uistate.T("txnperiod.lastMonth")),
				Option(Value("last-three-months"), SelectedIf(selectedPreset == "last-three-months"), uistate.T("txnperiod.lastThreeMonths")),
				Option(Value("this-year"), SelectedIf(selectedPreset == "this-year"), uistate.T("txnperiod.thisYear")),
				Option(Value("all-dates"), SelectedIf(selectedPreset == "all-dates"), uistate.T("txnperiod.allDates")),
				If(selectedPreset == "shown-month", Option(Value("shown-month"), Selected(true), monthLabel(scope.Anchor))),
				Option(Value("custom"), SelectedIf(selectedPreset == "custom"), customLabel)),
			If(selectedPreset == "custom", Button(css.Class("txn-period-edit"), Type("button"),
				Attr("data-testid", "txn-period-edit"), Attr("aria-label", uistate.T("txnperiod.editDates")),
				Title(uistate.T("txnperiod.editDates")), Attr("aria-expanded", ariaBool(custom)), Attr("aria-controls", "txn-period-custom"),
				OnClick(editDates), uiw.Icon(icon.Calendar, css.Class(tw.W4, tw.H4)))),
			If(custom, Div(css.Class("txn-period-custom"),
				Attr("id", "txn-period-custom"), Attr("data-testid", "txn-period-custom"), Attr("role", "group"), Attr("aria-label", uistate.T("txnperiod.chooseDates")),
				Span(css.Class("txn-period-custom-title"), uistate.T("txnperiod.chooseDates")),
				Label(css.Class("txn-period-date"), Span(uistate.T("txnperiod.from")),
					Input(Type("date"), Attr("data-testid", "txn-period-from"), Attr("aria-label", uistate.T("txnperiod.from")), Value(draftFrom.Get()), OnInput(onFrom))),
				Label(css.Class("txn-period-date"), Span(uistate.T("txnperiod.to")),
					Input(Type("date"), Attr("data-testid", "txn-period-to"), Attr("aria-label", uistate.T("txnperiod.to")), Value(draftTo.Get()), OnInput(onTo))),
				If(!validRange, Span(css.Class("txn-period-error"), uistate.T("txnperiod.invalidRange"))),
				Div(css.Class("txn-period-custom-actions"),
					Button(css.Class("txn-period-cancel"), Type("button"), Attr("data-testid", "txn-period-cancel"), OnClick(cancelDates), uistate.T("common.cancel")),
					Button(css.Class("txn-period-apply"), Type("button"), Attr("data-testid", "txn-period-apply"), Disabled(!validRange), OnClick(applyDates), uistate.T("txnperiod.applyDates"))),
			)),
		),
		Button(css.Class("txn-scope-step"), Type("button"),
			Attr("data-testid", "txn-scope-next"),
			Disabled(nextDisabled),
			Attr("aria-label", uistate.T("txnperiod.next")), Title(uistate.T("txnperiod.next")),
			OnClick(next), uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		If(hasDateFilter, Button(css.Class("txn-scope-clear"), Type("button"),
			Attr("data-testid", "txn-scope-clear"),
			Attr("aria-label", uistate.T("txnperiod.clearDates")), Title(uistate.T("txnperiod.clearDates")),
			OnClick(clearDates), uiw.Icon(icon.Close, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
	)
}

func txnScopePresetRange(value string, now time.Time) (from, to, key string, ok bool) {
	switch value {
	case "this-month":
		from, to = txnscope.MonthBounds(now)
		key, ok = txnfilter.PeriodThisMonth, true
	case "last-month":
		from, to = txnscope.MonthBounds(now.AddDate(0, -1, 0))
		key, ok = txnfilter.PeriodLastMonth, true
	case "last-three-months":
		from, _ = txnscope.MonthBounds(now.AddDate(0, -2, 0))
		_, to = txnscope.MonthBounds(now)
		key, ok = txnfilter.PeriodLastThreeMonths, true
	case "this-year":
		from = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		to = time.Date(now.Year(), time.December, 31, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		key, ok = txnfilter.PeriodThisYear, true
	case "all-dates":
		key, ok = txnfilter.PeriodAllDates, true
	}
	return from, to, key, ok
}

func txnScopeCustomRange(from, to string) (string, string, string) {
	return from, to, txnfilter.PeriodCustom
}

func txnScopeStepRange(scope txnscope.Scope, direction int) (from, to, key string) {
	from, to = scope.Step(direction)
	anchor := scope.Anchor.AddDate(0, direction, 0)
	key = txnfilter.MonthPeriodPreset(anchor.Year(), anchor.Month())
	return from, to, key
}

func txnScopePreset(f uistate.TxFilter, now time.Time) string {
	if f.PeriodPreset == txnfilter.PeriodCustom {
		return "custom"
	}
	if f.From == "" && f.To == "" {
		return "all-dates"
	}
	monthFrom, monthTo := txnscope.MonthBounds(now)
	if f.From == monthFrom && f.To == monthTo {
		return "this-month"
	}
	lastFrom, lastTo := txnscope.MonthBounds(now.AddDate(0, -1, 0))
	if f.From == lastFrom && f.To == lastTo {
		return "last-month"
	}
	threeFrom, _ := txnscope.MonthBounds(now.AddDate(0, -2, 0))
	if f.From == threeFrom && f.To == monthTo {
		return "last-three-months"
	}
	yearFrom := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	yearTo := time.Date(now.Year(), time.December, 31, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	if f.From == yearFrom && f.To == yearTo {
		return "this-year"
	}
	if txnscope.Of(f.From, f.To, now).Kind == txnscope.Month {
		return "shown-month"
	}
	return "custom"
}

// txnViewToggle renders the Ledger/Calendar switch on the secondary toolbar row.
func txnViewToggle(_ struct{}) ui.Node {
	viewAtom := uistate.UseTxnViewMode()
	calendar := viewAtom.Get() == uistate.TxnViewCalendar
	showLedger := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.TxnViewTable) }))
	showCalendar := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.TxnViewCalendar) }))
	return Div(css.Class("txn-viewtoggle"), Attr("role", "group"),
		Attr("aria-label", uistate.T("txnperiod.viewLabel")),
		Button(css.Class("txn-viewtoggle-btn"), Type("button"),
			Attr("data-testid", "txn-view-ledger"),
			Attr("aria-pressed", ariaBool(!calendar)), OnClick(showLedger),
			uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("txnperiod.viewLedger"))),
		Button(css.Class("txn-viewtoggle-btn"), Type("button"),
			Attr("data-testid", "txn-view-calendar"),
			Attr("aria-pressed", ariaBool(calendar)), OnClick(showCalendar),
			uiw.Icon(icon.Calendar, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("txnperiod.viewCal"))),
	)
}
