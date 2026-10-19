// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnscope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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
	customOpen := ui.UseState(preset == "custom")
	prev := ui.UseEvent(Prevent(func() {
		customOpen.Set(false)
		setScopeStep(-1)
	}))
	next := ui.UseEvent(Prevent(func() {
		customOpen.Set(false)
		setScopeStep(1)
	}))
	onPreset := ui.UseEvent(func(value string) {
		customOpen.Set(value == "custom")
		if from, to, key, ok := txnScopePresetRange(value, now); ok {
			setRange(from, to, key)
		}
	})
	onFrom := ui.UseEvent(func(value string) {
		from, to, key := txnScopeCustomRange(value, f.To)
		setRange(from, to, key)
	})
	onTo := ui.UseEvent(func(value string) {
		from, to, key := txnScopeCustomRange(f.From, value)
		setRange(from, to, key)
	})

	selectedPreset := preset
	if customOpen.Get() {
		selectedPreset = "custom"
	}
	custom := customOpen.Get()
	stepDisabled := scope.Kind == txnscope.All

	return Div(css.Class("txn-scopebar"), Attr("data-testid", "txn-scopebar"),
		Attr("role", "group"), Attr("aria-label", uistate.T("txnperiod.scopeAria", txnScopeLabel(scope))),
		Button(css.Class("txn-scope-step"), Type("button"),
			Attr("data-testid", "txn-scope-prev"),
			Disabled(stepDisabled),
			Attr("aria-label", uistate.T("txnperiod.prev")), Title(uistate.T("txnperiod.prev")),
			OnClick(prev), uiw.Icon(icon.ChevronLeft, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Div(css.Class("txn-period-control"),
			Select(css.Class("txn-period-select"), Attr("data-testid", "txn-period-select"),
				Attr("aria-label", uistate.T("txnperiod.periodLabel")), OnChange(onPreset),
				Option(Value("this-month"), SelectedIf(selectedPreset == "this-month"), uistate.T("txnperiod.thisMonth")),
				Option(Value("last-month"), SelectedIf(selectedPreset == "last-month"), uistate.T("txnperiod.lastMonth")),
				Option(Value("last-three-months"), SelectedIf(selectedPreset == "last-three-months"), uistate.T("txnperiod.lastThreeMonths")),
				Option(Value("this-year"), SelectedIf(selectedPreset == "this-year"), uistate.T("txnperiod.thisYear")),
				Option(Value("all-dates"), SelectedIf(selectedPreset == "all-dates"), uistate.T("txnperiod.allDates")),
				If(selectedPreset == "shown-month", Option(Value("shown-month"), Selected(true), monthLabel(scope.Anchor))),
				Option(Value("custom"), SelectedIf(selectedPreset == "custom"), uistate.T("txnperiod.customRangeChoice"))),
			If(custom, Div(css.Class("txn-period-custom"),
				Label(css.Class("txn-period-date"), Span(uistate.T("txnperiod.from")),
					Input(Type("date"), Attr("aria-label", uistate.T("txnperiod.from")), Value(f.From), OnChange(onFrom))),
				Label(css.Class("txn-period-date"), Span(uistate.T("txnperiod.to")),
					Input(Type("date"), Attr("aria-label", uistate.T("txnperiod.to")), Value(f.To), OnChange(onTo))),
			)),
		),
		Button(css.Class("txn-scope-step"), Type("button"),
			Attr("data-testid", "txn-scope-next"),
			Disabled(stepDisabled),
			Attr("aria-label", uistate.T("txnperiod.next")), Title(uistate.T("txnperiod.next")),
			OnClick(next), uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
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
