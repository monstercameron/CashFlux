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
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txnscope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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
	// MonthName is the month this square belongs to ("June 2026"), used to name a
	// leading/trailing day out loud (C603). A dimmed, disabled square is a visual
	// signal only; the accessible name has to carry the same fact.
	MonthName string
	Today     bool
	NetStr    string
	NetTone   string
	AriaLabel string
	Count     int
	Ghosts    []txncalendar.Ghost
	OnPick    func(dateKey string)
}

// txnCalendarWidget renders the month-grid calendar view. Each in-month day is a
// keyboard-reachable button showing the day's net amount and a dot per few
// transactions; recurring items appear as dimmed ghost labels on their due dates.
// The month it shows is the LEDGER'S period, stepped from the scope bar above it
// (C560); clicking a day narrows the ledger to that day and returns to the table.
func txnCalendarWidget(props txnCalendarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	viewAtom := uistate.UseTxnViewMode()
	filterAtom := uistate.UseTxFilter()
	pr := uistate.UsePrefs().Get()
	weekStart := pr.WeekStartWeekday()

	now := time.Now()
	// C560: the visible month comes from the LEDGER'S date scope, not from a private
	// calendar-month atom anchored on time.Now(). Those were separate states, so the
	// grid could show August while the ledger label and rows were in July, and paging
	// the calendar moved neither the rows nor the totals. One derivation now feeds the
	// grid, the label above it, and the rows — the calendar is a projection of the
	// ledger's scope rather than a second opinion about it.
	f := filterAtom.Get()
	scope := txnscope.Of(f.From, f.To, now)
	anchor := scope.Anchor

	// Keep recurring ghosts aligned with the same effective ledger scope. Their
	// definitions can be matched by account/category and account ownership, but
	// not by text, tags, source, or amount (those fields do not exist on Recurring).
	effectiveFilter, _ := f.WithOwnerLens(uistate.UseMemberLens())
	scopedRecurring := txncalendar.ScopeRecurring(props.App.Recurring(), effectiveFilter, props.App.Accounts())
	rates := currency.Rates{Base: props.Base, Rates: props.App.Settings().FXRates}
	weeks, projectionErr := txncalendar.MonthInBase(anchor, weekStart, props.Shown, scopedRecurring, rates)
	todayKey := txncalendar.DayKey(now)

	// Picking a day writes the ledger's own range, so the calendar and the scope bar
	// above it move exactly one piece of state between them.
	setRange := func(from, to string) {
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.From, x.To = from, to })
	}

	// pickDay narrows the ledger to a single day and returns to the table view.
	//
	// C603: it also SAYS SO. Clicking a square changed the date filter and swapped
	// the whole view out from under the user in one motion, with nothing naming the
	// filter it had just applied — the ledger simply reappeared, shorter, for
	// reasons the user had to reconstruct from the scope bar. The notice names the
	// day now, and the scope bar's "All dates" is the way back.
	pickDay := func(dateKey string) {
		prevFrom, prevTo := f.From, f.To
		setRange(dateKey, dateKey)
		viewAtom.Set(uistate.TxnViewTable)
		day := dateKey
		if d, err := dateutil.ParseDate(dateKey); err == nil {
			day = pr.FormatDate(d)
		}
		if prevFrom == "" && prevTo == "" {
			uistate.PostNotice(uistate.T("transactions.calPickedFromAll", day), false)
			return
		}
		uistate.PostNotice(uistate.T("transactions.calPicked", day), false)
	}

	// Weekday header row (localized short names in week-start order). It shares
	// the weekly grid with the totals so each total stays aligned to its week.
	head := []any{css.Class("txn-cal-week-head")}
	for i := 0; i < 7; i++ {
		wd := time.Weekday((int(weekStart) + i) % 7)
		head = append(head, Div(css.Class("cal-head"), wd.String()[:3]))
	}
	head = append(head, Div(css.Class("cal-head"), uistate.T("transactions.calWeekTotal")))
	weekRows := make([]any, 0, len(weeks))
	for _, week := range weeks {
		row := []any{css.Class("txn-cal-week-row")}
		for _, day := range week {
			row = append(row, ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
				DateKey: txncalendar.DayKey(day.Date),
				Day:     day.Date.Day(),
				InMonth: day.InMonth,
				// C603: which month this square actually belongs to. A leading or
				// trailing day is dimmed and disabled, but its accessible name was the
				// bare number — so "29, 30, 1, 2" read as one continuous month to
				// anyone not seeing the dimming, and to everyone as an ambiguous grid.
				MonthName: day.Date.Format("January 2006"),
				Today:     txncalendar.DayKey(day.Date) == todayKey,
				NetStr:    calNetStr(day.Stat, props.Base),
				NetTone:   calNetTone(day.Stat),
				AriaLabel: calDayAria(day.Date.Day(), day.Stat, props.Base),
				Count:     day.Stat.Count,
				Ghosts:    day.Ghosts,
				OnPick:    pickDay,
			}))
		}
		weekTotal := txncalendar.WeekStat(week)
		scheduled := 0
		for _, day := range week {
			scheduled += len(day.Ghosts)
		}
		spent, income := calWeekAmounts(week, props.Shown, rates)
		weekOf := week[0].Date.Format("Jan 2")
		future := dateutil.WeekStart(week[0].Date, weekStart).After(dateutil.WeekStart(now, weekStart))
		row = append(row, ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{
			Stat: weekTotal, Base: props.Base, Scheduled: scheduled,
			Spent: spent, Income: income, WeekOf: weekOf, Future: future,
		}))
		weekRows = append(weekRows, Div(row...))
	}

	// C560: the calendar keeps a caption, not a second stepper. It used to carry its
	// own ‹ month › + Today row, which — now that the scope bar above it steps the
	// same state — put two identical month controls one on top of the other. The
	// caption states the LEDGER'S scope, so a single day or a custom range reads as
	// itself here too rather than as a bare month name beside a grid the rows do not
	// fill. The testids stay on the caption and the grid so existing selectors resolve.
	// C603: the caption states the ledger's scope — but under "All dates", or any
	// range that is not exactly one month, that scope does not name the month the
	// GRID is drawing, and the user is left reading a wall of day numbers with no
	// month anywhere on screen. When the two differ, both are stated.
	gridMonth := anchor.Format("January 2006")
	caption := txnScopeLabel(scope)
	if scope.Kind != txnscope.Month {
		caption = uistate.T("transactions.calShowingMonth", caption, gridMonth)
	}
	nav := Div(css.Class("txn-cal-nav", "row"),
		Span(css.Class("txn-cal-month"), Attr("data-testid", "txn-cal-month"), caption),
	)
	if projectionErr != nil {
		return uiw.Widget(uiw.WidgetProps{
			ID: "txn-calendar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: Div(css.Class("txn-cal"), nav,
				Div(css.Class("text-dim"), Attr("role", "status"), Attr("data-testid", "txn-cal-fx-error"),
					uistate.T("transactions.calFxMissingRate"))),
		})
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-calendar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("txn-cal"), nav, Div(css.Class("txn-cal-weeks"), Div(head...), Fragment(weekRows...)),
			Div(css.Class("txn-cal-legend"), Attr("style", "display:flex;align-items:center;gap:.4rem"),
				Span(css.Class("txn-cal-legend-dots"), Attr("style", "display:inline-flex;align-items:center"),
					Span(css.Class("cal-dot"), Attr("style", "position:static;width:7px;height:7px;border-radius:50%;background:var(--accent)"))),
				Span(uistate.T("transactions.calDotsLegend"))),
			Div(css.Class("text-dim"), uistate.T("transactions.calScheduledFilterNote"))),
	})
}

type txnCalWeekTotalProps struct {
	Stat      txncalendar.DayStat
	Base      string
	Scheduled int
	Spent     int64
	Income    int64
	WeekOf    string
	Future    bool
}

func txnCalWeekTotal(props txnCalWeekTotalProps) ui.Node {
	spent, _ := fmtLedgerAmount(money.New(-props.Spent, props.Base), false)
	spentClass := "txn-cal-week-spent"
	if props.Spent == 0 {
		spentClass += " text-dim"
	} else {
		spentClass += " text-down"
	}
	lines := []any{Span(ClassStr(spentClass), uistate.T("r9c.weekSpent", spent))}
	if props.Income > 0 {
		income, _ := fmtLedgerAmount(money.New(props.Income, props.Base), false)
		lines = append(lines, Span(css.Class("txn-cal-week-income text-up"), uistate.T("r9c.weekIncome", strings.TrimPrefix(income, "+"))))
	}
	lastLine := ""
	if props.Stat.Count > 0 {
		lastLine = uistate.T("r9c.weekTransactions", props.Stat.Count)
		if props.Stat.Count == 1 {
			lastLine = uistate.T("r9c.weekOneTransaction")
		}
	} else if props.Future && props.Scheduled > 0 {
		lastLine = uistate.T("r6f.weekScheduled", props.Scheduled)
	} else if props.Future {
		lastLine = uistate.T("r9c.weekNothingYet")
	} else if props.Scheduled > 0 {
		lastLine = uistate.T("r6f.weekScheduled", props.Scheduled)
	} else {
		lastLine = uistate.T("transactions.calWeekCount", 0)
	}
	lastClass := "txn-cal-week-count"
	if props.Stat.Count == 0 && props.Scheduled == 0 {
		lastClass += " text-dim"
	}
	lines = append(lines, Span(ClassStr(lastClass), lastLine))
	ariaSpent := strings.TrimPrefix(spent, "−")
	aria := uistate.T("r9c.weekAria", props.WeekOf, ariaSpent, lastLine)
	if props.Income > 0 {
		income, _ := fmtLedgerAmount(money.New(props.Income, props.Base), false)
		aria = uistate.T("r9c.weekAriaIncome", props.WeekOf, ariaSpent, strings.TrimPrefix(income, "+"), lastLine)
	}
	return Div(css.Class("txn-cal-week-total"),
		Attr("data-testid", "txn-cal-week-total"),
		Attr("aria-label", aria),
		Attr("title", aria),
		Fragment(lines...),
	)
}

// calWeekAmounts totals signed transaction amounts into separate spending and
// income figures for the dates represented by the week. The caller has already
// projected this same scoped transaction set through these rates successfully.
func calWeekAmounts(week []txncalendar.Cell, txns []domain.Transaction, rates currency.Rates) (spent, income int64) {
	days := make(map[string]struct{}, len(week))
	for _, day := range week {
		if day.InMonth {
			days[txncalendar.DayKey(day.Date)] = struct{}{}
		}
	}
	for _, txn := range txns {
		if _, ok := days[txncalendar.DayKey(txn.Date)]; !ok || txn.TransferAccountID != "" {
			continue
		}
		converted, err := rates.Convert(txn.Amount, rates.Base)
		if err != nil {
			continue
		}
		if converted.Amount < 0 {
			spent -= converted.Amount
		} else if converted.Amount > 0 {
			income += converted.Amount
		}
	}
	return spent, income
}

// calNetStr formats a day's net minor-unit amount for the cell, or "" for a day
// with no transactions (a blank cell reads calmer than "$0.00").
func calNetStr(s txncalendar.DayStat, base string) string {
	if s.Count == 0 {
		return ""
	}
	text, _ := fmtLedgerAmount(money.New(s.Net, base), false)
	return text
}

// calNetTone colors positive net income green and expenses red.
func calNetTone(s txncalendar.DayStat) string {
	switch {
	case s.Net > 0:
		return "text-up"
	case s.Net < 0:
		return "text-down"
	default:
		return ""
	}
}

func calDayAria(day int, s txncalendar.DayStat, base string) string {
	amount, _ := fmtLedgerAmount(money.New(s.Net, base), false)
	switch {
	case s.Count == 0:
		return uistate.T("transactions.calDayEmpty", strconv.Itoa(day))
	case s.Net < 0:
		return uistate.T("transactions.calDaySpent", strconv.Itoa(day), strings.TrimPrefix(amount, "−"))
	case s.Net > 0:
		return uistate.T("transactions.calDayNet", strconv.Itoa(day), amount)
	default:
		return uistate.T("transactions.calDayZero", strconv.Itoa(day), amount)
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

	label := props.AriaLabel
	if label == "" {
		label = uistate.T("transactions.calDayLabel", strconv.Itoa(props.Day), props.NetStr)
	}
	// C603: a leading/trailing square names its own month and says it is not part
	// of the month on show, so the grid's edges are unambiguous by reading as well
	// as by dimming.
	if !props.InMonth && props.MonthName != "" {
		label = uistate.T("transactions.calDayOtherMonth", strconv.Itoa(props.Day), props.MonthName)
	}
	args := []any{
		ClassStr(cls),
		Type("button"),
		Attr("data-date", props.DateKey),
		Attr("data-testid", "txn-cal-day-"+props.DateKey),
		Attr("aria-label", label),
		Attr("title", label),
	}
	if props.Today {
		// Shared calendar styling gives .cal-cell.today a filled wash, which makes
		// today's marker read like a selected day. Keep it as an outline only.
		args = append(args, Attr("style", "outline:2px solid var(--accent);outline-offset:-2px;background:var(--bg-card);box-shadow:none"))
	}
	if props.InMonth {
		args = append(args, OnClick(pick))
		if props.Today {
			args = append(args, Attr("aria-current", "date"))
		}
	} else {
		args = append(args, Disabled(true))
	}
	args = append(args,
		Span(css.Class("cal-day"), strconv.Itoa(props.Day)),
		If(props.NetStr != "", Span(ClassStr("txn-cal-net "+props.NetTone), props.NetStr)),
		Div(css.Class("txn-cal-dots"), Fragment(dotNodes...)),
		Fragment(ghostNodes...),
	)
	return Button(args...)
}
