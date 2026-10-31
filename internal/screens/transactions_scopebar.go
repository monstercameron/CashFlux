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
	idBase := ui.UseId()
	controlID, customID := idBase+"-control", idBase+"-custom"
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
	onPreset := func(value string) {
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
	}
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
			control := doc.Call("getElementById", controlID)
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
	periodOptions := []uiw.SelectOption{
		{Value: "this-month", Label: uistate.T("txnperiod.thisMonth")},
		{Value: "last-month", Label: uistate.T("txnperiod.lastMonth")},
		{Value: "last-three-months", Label: uistate.T("txnperiod.lastThreeMonths")},
		{Value: "this-year", Label: uistate.T("txnperiod.thisYear")},
		{Value: "all-dates", Label: uistate.T("txnperiod.allDates")},
	}
	if selectedPreset == "shown-month" {
		periodOptions = append(periodOptions, uiw.SelectOption{Value: "shown-month", Label: monthLabel(scope.Anchor)})
	}
	periodOptions = append(periodOptions, uiw.SelectOption{Value: "custom", Label: customLabel})
	controlClass := "txn-period-control"
	if selectedPreset == "custom" {
		controlClass += " has-edit"
	}
	prevDisabled := scope.Kind == txnscope.All
	nextDisabled := !scope.CanStepForward(now)
	hasDateFilter := f.From != "" || f.To != ""

	prevLabel, nextLabel := uistate.T("txnperiod.prev"), uistate.T("txnperiod.next")
	return Div(css.Class("txn-scopebar"), Attr("data-testid", "txn-scopebar"),
		Attr("role", "group"), Attr("aria-label", uistate.T("txnperiod.scopeAria", txnScopeLabel(scope))),
		uiw.ToolButton(uiw.ToolButtonProps{Icon: icon.ChevronLeft, AriaLabel: prevLabel, Title: prevLabel, TestID: "txn-scope-prev"},
			Disabled(prevDisabled), OnClick(prev)),
		Div(ClassStr(controlClass), Attr("id", controlID),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   periodOptions,
				Selected:  selectedPreset,
				AriaLabel: uistate.T("txnperiod.periodLabel"),
				Class:     "txn-period-select",
				TestID:    "txn-period-select",
				OnChange:  onPreset,
			}),
			If(selectedPreset == "custom", uiw.ToolButton(uiw.ToolButtonProps{Icon: icon.Calendar, AriaLabel: uistate.T("txnperiod.editDates"), Title: uistate.T("txnperiod.editDates"), TestID: "txn-period-edit", Open: custom},
				Attr("aria-expanded", ariaBool(custom)), Attr("aria-controls", customID), OnClick(editDates))),
			If(custom, Div(css.Class("txn-period-custom"),
				Attr("id", customID), Attr("data-testid", "txn-period-custom"), Attr("role", "group"), Attr("aria-label", uistate.T("txnperiod.chooseDates")),
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
		uiw.ToolButton(uiw.ToolButtonProps{Icon: icon.ChevronRight, AriaLabel: nextLabel, Title: nextLabel, TestID: "txn-scope-next"},
			Disabled(nextDisabled), OnClick(next)),
		If(hasDateFilter, uiw.ToolButton(uiw.ToolButtonProps{Icon: icon.Close, AriaLabel: uistate.T("txnperiod.clearDates"), Title: uistate.T("txnperiod.clearDates"), TestID: "txn-scope-clear"},
			OnClick(clearDates))),
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

// txnViewToggle renders the Ledger/Calendar switch on the secondary toolbar row. It is the
// shared RadioGroup the Accounts page uses for its single-choice views.
func txnViewToggle(_ struct{}) ui.Node {
	viewAtom := uistate.UseTxnViewMode()
	return uiw.RadioGroup(uiw.RadioGroupProps{
		Label: uistate.T("txnperiod.viewLabel"),
		Value: viewAtom.Get(),
		Options: []uiw.RadioOption{
			{Value: uistate.TxnViewTable, Label: uistate.T("txnperiod.viewLedger"), TestID: "txn-view-ledger"},
			{Value: uistate.TxnViewCalendar, Label: uistate.T("txnperiod.viewCal"), TestID: "txn-view-calendar"},
		},
		OnChange: func(v string) { viewAtom.Set(v) },
	})
}
