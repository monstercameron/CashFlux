// SPDX-License-Identifier: MIT

//go:build js && wasm

// Transactions calendar view (TX8) and register-mode helpers (TX12). The calendar
// is a month grid rendered as a Native tile in the /transactions surface; it is a
// pure projection of the already-filtered ledger (RenderCtx.ScopedTxns), so active
// filter chips scope it exactly as they scope the table. Clicking a day narrows the
// filter to that single day and returns to the table view. Recurring cash flows
// project forward as dimmed ghost markers on their due dates.
package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// balanceStr returns the pre-formatted running balance for a transaction id in
// register mode, or "" when there is no figure (runBal nil, or the id absent).
func balanceStr(runBal map[string]money.Money, id string) string {
	if runBal == nil {
		return ""
	}
	if m, ok := runBal[id]; ok {
		return fmtMoney(m)
	}
	return ""
}

// balanceTone colours a running balance red when it has gone negative (the account
// dipped below zero after this row), and leaves it neutral otherwise — a register
// balance is a neutral figure, not a gain/loss, so only the danger case is painted.
func balanceTone(runBal map[string]money.Money, id string) string {
	if runBal == nil {
		return ""
	}
	if m, ok := runBal[id]; ok && m.IsNegative() {
		return "text-down"
	}
	return ""
}

// txnCalendarProps carries the data the calendar tile needs: the app (for
// recurrings and base currency), the base currency, and the filtered set the
// calendar buckets.
type txnCalendarProps struct {
	App   *appstate.App
	Base  string
	Shown []domain.Transaction
}

// txnCalMonthCellProps configures one day button in the calendar grid. Callbacks
// are plain funcs so the per-row component owns its own click hook (GWC rule).
type txnCalMonthCellProps struct {
	DateKey string // YYYY-MM-DD, the filter value applied on click
	Day     int
	InMonth bool
	Today   bool
	NetStr  string
	NetTone string
	Count   int
	Ghosts  []txncalendar.Ghost
	OnPick  func(dateKey string)
}

// txnCalendarWidget renders the month-grid calendar view. Each in-month day is a
// keyboard-reachable button showing the day's net amount and a dot per few
// transactions; recurring items appear as dimmed ghost labels on their due dates.
// Prev/next page the visible month. Clicking a day filters the ledger to that day
// and switches back to the table.
func txnCalendarWidget(props txnCalendarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	monthAtom := uistate.UseTxnCalMonth()
	viewAtom := uistate.UseTxnViewMode()
	filterAtom := uistate.UseTxFilter()
	pr := uistate.UsePrefs().Get()
	weekStart := pr.WeekStartWeekday()

	now := time.Now()
	anchor := monthAtom.Get()
	if anchor.IsZero() {
		anchor = now
	}

	weeks := txncalendar.Month(anchor, weekStart, props.Shown, props.App.Recurring())
	todayKey := txncalendar.DayKey(now)

	// Month paging: prev/next shift the anchor by a calendar month.
	prevMonth := ui.UseEvent(Prevent(func() { monthAtom.Set(dateutil.AddMonths(dateutil.MonthStart(anchor), -1)) }))
	nextMonth := ui.UseEvent(Prevent(func() { monthAtom.Set(dateutil.AddMonths(dateutil.MonthStart(anchor), 1)) }))
	today := ui.UseEvent(Prevent(func() { monthAtom.Set(time.Time{}) }))

	// pickDay narrows the ledger to a single day and returns to the table view.
	pickDay := func(dateKey string) {
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.From, x.To = dateKey, dateKey })
		viewAtom.Set(uistate.TxnViewTable)
	}

	// Weekday header row (localized short names in week-start order).
	head := []any{css.Class("cal-grid txn-cal-grid")}
	for i := 0; i < 7; i++ {
		wd := time.Weekday((int(weekStart) + i) % 7)
		head = append(head, Div(css.Class("cal-head"), wd.String()[:3]))
	}
	for _, week := range weeks {
		for _, day := range week {
			head = append(head, ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
				DateKey: txncalendar.DayKey(day.Date),
				Day:     day.Date.Day(),
				InMonth: day.InMonth,
				Today:   txncalendar.DayKey(day.Date) == todayKey,
				NetStr:  calNetStr(day.Stat, props.Base),
				NetTone: calNetTone(day.Stat),
				Count:   day.Stat.Count,
				Ghosts:  day.Ghosts,
				OnPick:  pickDay,
			}))
		}
	}

	nav := Div(css.Class("txn-cal-nav", "row"),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("aria-label", uistate.T("transactions.calPrevMonth")), Attr("data-testid", "txn-cal-prev"), OnClick(prevMonth), uiw.Icon(icon.ChevronLeft, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Span(css.Class("txn-cal-month"), monthLabel(dateutil.MonthStart(anchor))),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("aria-label", uistate.T("transactions.calNextMonth")), Attr("data-testid", "txn-cal-next"), OnClick(nextMonth), uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
		Button(css.Class("btn btn-sm btn-tool"), Type("button"), Attr("data-testid", "txn-cal-today"), OnClick(today), uistate.T("transactions.calToday")),
	)

	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-calendar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("txn-cal"), nav, Div(head...)),
	})
}

// calNetStr formats a day's net minor-unit amount for the cell, or "" for a day
// with no transactions (a blank cell reads calmer than "$0.00").
func calNetStr(s txncalendar.DayStat, base string) string {
	if s.Count == 0 {
		return ""
	}
	return fmtMoney(money.New(s.Net, base))
}

// calNetTone colours the day net: red for net spend (negative), green for net
// income (positive), neutral for zero.
func calNetTone(s txncalendar.DayStat) string {
	switch {
	case s.Net < 0:
		return "text-down"
	case s.Net > 0:
		return "text-up"
	default:
		return ""
	}
}

// txnCalMonthCell renders one day button. It owns its click hook so it is safe
// inside the grid loop (GWC rule). Out-of-month padding days are inert.
func txnCalMonthCell(props txnCalMonthCellProps) ui.Node {
	pick := ui.UseEvent(func() {
		if props.InMonth && props.OnPick != nil {
			props.OnPick(props.DateKey)
		}
	})

	cls := "cal-cell txn-cal-cell"
	if !props.InMonth {
		cls += " out"
	}
	if props.Today {
		cls += " today"
	}

	// A dot per ~5 transactions conveys density without crowding; at least one dot
	// when there is any activity.
	dots := 0
	if props.Count > 0 {
		dots = (props.Count + 4) / 5
		if dots < 1 {
			dots = 1
		}
		if dots > 5 {
			dots = 5
		}
	}
	dotNodes := make([]any, 0, dots)
	for i := 0; i < dots; i++ {
		dotNodes = append(dotNodes, Span(css.Class("cal-dot")))
	}

	// Ghost markers: dimmed labels for recurrings due this day, no interaction beyond
	// a title/tooltip.
	ghostNodes := make([]any, 0, len(props.Ghosts))
	for _, g := range props.Ghosts {
		ghostNodes = append(ghostNodes, Span(css.Class("txn-cal-ghost text-dim"), Attr("title", g.Label), g.Label))
	}

	label := uistate.T("transactions.calDayLabel", strconv.Itoa(props.Day), props.NetStr)
	args := []any{
		ClassStr(cls),
		Type("button"),
		Attr("data-date", props.DateKey),
		Attr("data-testid", "txn-cal-day-"+props.DateKey),
		Attr("aria-label", label),
	}
	if props.InMonth {
		args = append(args, OnClick(pick))
	} else {
		args = append(args, Disabled(true))
	}
	args = append(args,
		Span(css.Class("cal-day"), strconv.Itoa(props.Day)),
		If(props.NetStr != "", Span(ClassStr("txn-cal-net "+tw.ColorClass(props.NetTone)), props.NetStr)),
		Div(css.Class("txn-cal-dots"), Fragment(dotNodes...)),
		Fragment(ghostNodes...),
	)
	return Button(args...)
}
