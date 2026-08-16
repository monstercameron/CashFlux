// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/txnscope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnScopeLabel renders a ledger scope as the sentence the bar shows. It is what
// makes the label incapable of disagreeing with the rows: every branch is derived
// from the same From/To the ledger filters on, so a scope that is not a whole month
// says what it really is instead of naming a month it only partly covers.
func txnScopeLabel(s txnscope.Scope) string {
	switch s.Kind {
	case txnscope.All:
		return uistate.T("txnperiod.allLabel")
	case txnscope.Month:
		return monthLabel(s.Anchor)
	case txnscope.Day:
		return uistate.T("txnperiod.oneDay", s.From.Format("Jan 2, 2006"))
	default:
		from, to := "…", "…"
		if !s.From.IsZero() {
			from = s.From.Format("Jan 2, 2006")
		}
		if !s.To.IsZero() {
			to = s.To.Format("Jan 2, 2006")
		}
		return uistate.T("txnperiod.customRange", from, to)
	}
}

// txnScopeBar is the ledger's period + view bar (C560).
//
// The page used to show the top bar's reporting-period pill, which stores a snapped
// period.Window that cannot express what the ledger's date filter can — so the pill
// could read "Jul 2026" over August rows, and stepping it moved neither the rows nor
// the totals nor the calendar. This bar replaces it with a control over the ledger's
// OWN scope: the label is derived from the same From/To the rows are filtered by, so
// the two are the same fact rendered twice rather than two states hoping to agree.
//
// It also owns the Ledger/Calendar switch. That was an entry buried in "⋯ More" with
// no return path, which is how a user ends up in the calendar unsure how to leave; a
// two-button segmented pair with a pressed state says which view is showing and
// makes the way back a single visible click.
// It takes an empty props bag so it can be mounted with ui.CreateElement, which
// isolates its hooks from the toolbar tile that hosts it.
func txnScopeBar(_ struct{}) ui.Node {
	filterAtom := uistate.UseTxFilter()
	viewAtom := uistate.UseTxnViewMode()
	f := filterAtom.Get()
	now := time.Now()
	scope := txnscope.Of(f.From, f.To, now)
	calendar := viewAtom.Get() == uistate.TxnViewCalendar

	setRange := func(from, to string) {
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.From, x.To = from, to })
	}
	prev := ui.UseEvent(Prevent(func() { setRange(scope.Step(-1)) }))
	next := ui.UseEvent(Prevent(func() { setRange(scope.Step(1)) }))
	thisMonth := ui.UseEvent(Prevent(func() { setRange(txnscope.MonthBounds(time.Now())) }))
	allDates := ui.UseEvent(Prevent(func() { setRange("", "") }))
	showLedger := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.TxnViewTable) }))
	showCalendar := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.TxnViewCalendar) }))

	label := txnScopeLabel(scope)
	// "This month" is offered only when it would change something, and "All dates"
	// only when there is a bound to drop — a control that cannot do anything is
	// noise, and here it would also read as a state the ledger is already in.
	atCurrent := scope.IsCurrentMonth(now)
	bounded := scope.Kind != txnscope.All

	return Div(css.Class("txn-scopebar"), Attr("data-testid", "txn-scopebar"),
		Attr("role", "group"), Attr("aria-label", uistate.T("txnperiod.scopeAria", label)),
		Button(css.Class("txn-scope-step"), Type("button"),
			Attr("data-testid", "txn-scope-prev"),
			Attr("aria-label", uistate.T("txnperiod.prev")), Title(uistate.T("txnperiod.prev")),
			OnClick(prev), uiw.Icon(icon.ChevronLeft, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		// aria-live so stepping the month announces the new scope; the rows below are
		// the thing that changed, and this line is their name.
		Span(css.Class("txn-scope-label"), Attr("data-testid", "txn-scope-label"),
			Attr("aria-live", "polite"), label),
		Button(css.Class("txn-scope-step"), Type("button"),
			Attr("data-testid", "txn-scope-next"),
			Attr("aria-label", uistate.T("txnperiod.next")), Title(uistate.T("txnperiod.next")),
			OnClick(next), uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		If(!atCurrent, Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "txn-scope-thismonth"), OnClick(thisMonth),
			uistate.T("txnperiod.thisMonth"))),
		If(bounded, Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "txn-scope-alldates"), OnClick(allDates),
			uistate.T("txnperiod.allDates"))),
		Div(css.Class("txn-scopebar-spacer")),
		Div(css.Class("txn-viewtoggle"), Attr("role", "group"),
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
				Span(uistate.T("txnperiod.viewCal")))),
	)
}
