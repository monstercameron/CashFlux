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
	MovedStr  string
	AriaLabel string
	Count     int
	Ghosts    []txncalendar.Ghost
	Rates     currency.Rates
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

	now := dateutil.WallClockUTC(time.Now())
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
	var projection txncalendar.CashflowMonth
	var projectionErr error
	var liabilityAccounts []domain.Account
	if strings.TrimSpace(effectiveFilter.Categories) == "" && strings.TrimSpace(effectiveFilter.Category) == "" {
		liabilityAccounts = txncalendar.ScopeLiabilityAccounts(props.App.Accounts(), effectiveFilter)
		projection, projectionErr = txncalendar.MonthCashflowWithAccounts(anchor, weekStart, props.Shown, scopedRecurring, liabilityAccounts, rates)
	} else {
		projection, projectionErr = txncalendar.MonthCashflow(anchor, weekStart, props.Shown, scopedRecurring, rates)
	}
	weeks := projection.Weeks
	showScheduled := strings.TrimSpace(f.Text) == ""
	if showScheduled {
		settled := props.App.BillMatchPaidOccurrences()
		for key := range txncalendar.SettledLiabilityOccurrences(projection.Weeks, props.App.Transactions(), liabilityAccounts, now) {
			settled[key] = true
		}
		weeks = txncalendar.FoldSettled(weeks, settled, now)
	}
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
		head = append(head, Div(css.Class("cal-head"), Attr("role", "columnheader"), Attr("aria-label", calendarWeekday(wd, true)), calendarWeekday(wd, false)))
	}
	head = append(head, Div(css.Class("cal-head"), uistate.T("transactions.calWeekTotal")))
	weekRows := make([]any, 0, len(weeks))
	for _, week := range weeks {
		row := []any{css.Class("txn-cal-week-row")}
		for _, day := range week.Days {
			row = append(row, ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
				DateKey: txncalendar.DayKey(day.Date),
				Day:     day.Date.Day(),
				InMonth: day.InMonth,
				// C603: which month this square actually belongs to. A leading or
				// trailing day is dimmed and disabled, but its accessible name was the
				// bare number — so "29, 30, 1, 2" read as one continuous month to
				// anyone not seeing the dimming, and to everyone as an ambiguous grid.
				MonthName: calendarMonthYear(day.Date),
				Today:     txncalendar.DayKey(day.Date) == todayKey,
				NetStr:    calDayNetStr(day.Cashflow, props.Base),
				NetTone:   calNetTone(txncalendar.DayStat{Net: day.Cashflow.Net, Count: day.Cashflow.Count}),
				MovedStr:  calMovedStr(day.Cashflow.Moved, props.Base),
				AriaLabel: calDayAria(day.Date.Day(), day.Cashflow, props.Base),
				Count:     day.Cashflow.Count,
				Ghosts:    scheduledGhosts(day.Ghosts, showScheduled),
				Rates:     rates,
				OnPick:    pickDay,
			}))
		}
		scheduled := 0
		for _, day := range week.Days {
			if showScheduled {
				scheduled += len(day.Ghosts)
			}
		}
		weekOf := pr.FormatDate(week.Days[0].Date)
		future := dateutil.WeekStart(week.Days[0].Date, weekStart).After(dateutil.WeekStart(now, weekStart))
		row = append(row, ui.CreateElement(txnCalWeekTotal, txnCalWeekTotalProps{
			Cashflow: week.Total, Base: props.Base, Scheduled: scheduled,
			WeekOf: weekOf, Future: future, SummaryID: "txn-cal-week-" + txncalendar.DayKey(week.Days[0].Date),
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
	gridMonth := calendarMonthYear(anchor)
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

	fxNotice := If(len(projection.RateIssues) > 0,
		Div(css.Class("text-dim"), Attr("role", "status"), Attr("data-testid", "txn-cal-fx-warning"),
			uistate.T("transactions.tq3.monthFxMissing", len(projection.RateIssues))))
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-calendar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("txn-cal"), nav, fxNotice, Div(css.Class("txn-cal-weeks"), Div(head...), Fragment(weekRows...)),
			Div(css.Class("txn-cal-legend"),
				Span(css.Class("txn-cal-legend-dots"),
					Span(css.Class("cal-dot txn-cal-legend-dot"))),
				Span(uistate.T("transactions.calDotsLegend"))),
			If(showScheduled, Div(css.Class("text-dim"), uistate.T("transactions.calDueFilterNote")))),
	})
}

func scheduledGhosts(ghosts []txncalendar.Ghost, visible bool) []txncalendar.Ghost {
	if !visible {
		return nil
	}
	return ghosts
}

type txnCalWeekTotalProps struct {
	Cashflow  txncalendar.Cashflow
	Base      string
	Scheduled int
	WeekOf    string
	Future    bool
	SummaryID string
}

func txnCalWeekTotal(props txnCalWeekTotalProps) ui.Node {
	spent, _ := fmtLedgerAmount(money.New(props.Cashflow.Spent, props.Base), false)
	spentClass := "txn-cal-week-spent"
	if props.Cashflow.Spent == 0 {
		spentClass += " text-dim"
	} else {
		spentClass += " text-down"
	}
	lines := make([]any, 0, 4)
	if props.Cashflow.Spent > 0 {
		lines = append(lines, Span(ClassStr(spentClass), uistate.T("r9c.weekSpent", strings.TrimPrefix(spent, "+"))))
	} else if props.Scheduled > 0 || props.Future {
		// Keep the older future-week summary discoverable to assistive technology
		// while the visual cell stays free of a misleading zero-spend claim.
		lines = append(lines, Span(css.Class("sr-only txn-cal-empty-week text-dim"), uistate.T("r9c.weekSpent", strings.TrimPrefix(spent, "+"))))
	}
	if props.Cashflow.Income > 0 {
		income, _ := fmtLedgerAmount(money.New(props.Cashflow.Income, props.Base), false)
		lines = append(lines, Span(css.Class("txn-cal-week-income text-up"), uistate.T("r9c.weekIncome", strings.TrimPrefix(income, "+"))))
	}
	if props.Cashflow.Moved > 0 {
		moved, _ := fmtLedgerAmount(money.New(props.Cashflow.Moved, props.Base), false)
		lines = append(lines, Span(css.Class("txn-cal-week-moved"), uistate.T("transactions.tq3.weekMoved", strings.TrimPrefix(moved, "+"))))
	}
	lastLine := weekTotalLastLine(props)
	lastClass := "txn-cal-week-count"
	if props.Cashflow.Count == 0 && props.Scheduled == 0 {
		lastClass += " text-dim"
	}
	lines = append(lines, Span(ClassStr(lastClass), lastLine))
	return txnCalWeekTotalNode(props, lines)
}

func weekTotalLastLine(props txnCalWeekTotalProps) string {
	if props.Cashflow.Count > 0 {
		if props.Cashflow.Count == 1 {
			return uistate.T("transactions.tq3.weekOne")
		}
		return uistate.T("transactions.tq3.weekCount", props.Cashflow.Count)
	}
	if props.Scheduled > 0 {
		return uistate.T("transactions.tq3.weekScheduled", props.Scheduled)
	}
	if props.Future {
		return uistate.T("transactions.tq3.weekEmpty")
	}
	return "\u2003"
}

func txnCalWeekTotalNode(props txnCalWeekTotalProps, lines []any) ui.Node {
	spent, _ := fmtLedgerAmount(money.New(props.Cashflow.Spent, props.Base), false)
	ariaSpent := strings.TrimPrefix(spent, "+")
	income, _ := fmtLedgerAmount(money.New(props.Cashflow.Income, props.Base), false)
	moved, _ := fmtLedgerAmount(money.New(props.Cashflow.Moved, props.Base), false)
	countSummary := uistate.T("transactions.tq3.weekManySummary", props.Cashflow.Count)
	if props.Cashflow.Count == 1 {
		countSummary = uistate.T("transactions.tq3.weekOneSummary")
	}
	scheduledSummary := ""
	if props.Scheduled > 0 {
		scheduledSummary = uistate.T("transactions.tq3.weekSummaryScheduled", props.Scheduled)
	}
	aria := uistate.T("transactions.tq3.weekSummary", props.WeekOf, ariaSpent, strings.TrimPrefix(income, "+"), strings.TrimPrefix(moved, "+"), countSummary, scheduledSummary)
	return Div(css.Class("txn-cal-week-total"),
		Attr("data-testid", "txn-cal-week-total"),
		Attr("aria-describedby", props.SummaryID),
		Span(css.Class("sr-only"), Attr("id", props.SummaryID), aria),
		Fragment(lines...),
	)
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

func calDayNetStr(cash txncalendar.Cashflow, base string) string {
	if cash.Count == 0 || (cash.Spent == 0 && cash.Income == 0) {
		return ""
	}
	return calNetStr(txncalendar.DayStat{Net: cash.Net, Count: cash.Count}, base)
}

func calMovedStr(amount int64, base string) string {
	if amount <= 0 {
		return ""
	}
	formatted, _ := fmtLedgerAmount(money.New(amount, base), false)
	return uistate.T("transactions.tq3.dayMoved", strings.TrimPrefix(formatted, "+"))
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

func calDayAria(day int, cash txncalendar.Cashflow, base string) string {
	if cash.Count == 0 {
		return uistate.T("transactions.tq3.dayEmpty", strconv.Itoa(day))
	}
	spent, _ := fmtLedgerAmount(money.New(cash.Spent, base), false)
	income, _ := fmtLedgerAmount(money.New(cash.Income, base), false)
	moved, _ := fmtLedgerAmount(money.New(cash.Moved, base), false)
	count := uistate.T("transactions.tq3.dayMany", cash.Count)
	if cash.Count == 1 {
		count = uistate.T("transactions.tq3.dayOne")
	}
	return uistate.T("transactions.tq3.dayCashflow", strconv.Itoa(day), strings.TrimPrefix(spent, "+"), strings.TrimPrefix(income, "+"), strings.TrimPrefix(moved, "+"), count)
}

func calendarMonthYear(t time.Time) string {
	return budgetMonthName(t, false) + " " + strconv.Itoa(t.Year())
}

func calendarWeekday(day time.Weekday, full bool) string {
	keys := [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	suffix := ""
	if full {
		suffix = ".full"
	}
	return uistate.T("transactions.tq3.weekday." + keys[int(day)] + suffix)
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
	dots := calTransactionDotCount(props.Count)
	dotNodes := make([]any, 0, dots)
	for i := 0; i < dots; i++ {
		dotNodes = append(dotNodes, Span(css.Class("cal-dot")))
	}

	// Ghost markers: dimmed labels for recurrings due this day, no interaction beyond
	// a title/tooltip.
	ghostNodes := make([]any, 0, len(props.Ghosts))
	ghostAriaLabels := make([]string, 0, len(props.Ghosts))
	for _, g := range props.Ghosts {
		amount := money.New(g.Amount, g.Currency)
		amountValid := true
		var amountErr error
		amount.Amount, amountErr = money.Abs(amount.Amount)
		amountValid = amountErr == nil
		amountText := uistate.T("transactions.calScheduledAmountUnavailable")
		if amountValid {
			converted, err := props.Rates.Convert(amount, props.Rates.Base)
			if err == nil {
				amountText, _ = fmtLedgerAmount(converted, false)
				amountText = strings.TrimPrefix(amountText, "+")
			}
		}
		className := "txn-cal-ghost text-dim"
		if g.NotPosted {
			className = "txn-cal-ghost txn-cal-ghost-missed"
		}
		ghostAriaLabels = append(ghostAriaLabels, ghostSummaryAria(g, amountText))
		if g.NotPosted {
			ghostNodes = append(ghostNodes, Span(css.Class(className),
				Span(css.Class("txn-cal-ghost-status"), uistate.T("transactions.calNotPostedStatus", amountText)),
				Span(css.Class("txn-cal-ghost-label"), g.Label)))
		} else {
			ghostNodes = append(ghostNodes, Span(css.Class(className),
				Span(css.Class("txn-cal-ghost-status"), uistate.T("transactions.calDueStatus", amountText)),
				Span(css.Class("txn-cal-ghost-label"), g.Label)))
		}
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
	if len(ghostAriaLabels) > 0 {
		label = uistate.T("transactions.calDayWithScheduled", label, strings.Join(ghostAriaLabels, "; "))
	}
	args := []any{
		ClassStr(cls),
		Type("button"),
		Attr("data-date", props.DateKey),
		Attr("data-testid", "txn-cal-day-"+props.DateKey),
		Attr("aria-label", label),
	}
	click := OnClick(pick)
	if props.InMonth {
		args = append(args, click)
		if props.Today {
			args = append(args, Attr("aria-current", "date"))
		}
	} else {
		args = append(args, Disabled(true))
	}
	args = append(args,
		Span(css.Class("cal-day"), strconv.Itoa(props.Day)),
		If(props.NetStr != "", Span(ClassStr("txn-cal-net "+props.NetTone), props.NetStr)),
		If(props.MovedStr != "", Span(css.Class("txn-cal-moved text-dim"), props.MovedStr)),
		Span(css.Class("txn-cal-dots"), Fragment(dotNodes...)),
		Fragment(ghostNodes...),
	)
	return Button(args...)
}

func ghostSummaryAria(ghost txncalendar.Ghost, amountText string) string {
	switch {
	case ghost.NotPosted:
		return uistate.T("transactions.calNotPostedSummary", ghost.Label, amountText)
	case ghost.Liability:
		return uistate.T("transactions.calAccountDueSummary", ghost.Label, amountText)
	default:
		return uistate.T("transactions.calScheduledDueSummary", ghost.Label, amountText)
	}
}

func calTransactionDotCount(count int) int {
	if count <= 0 {
		return 0
	}
	dots := 1 + (count-1)/5
	if dots > 5 {
		return 5
	}
	return dots
}
