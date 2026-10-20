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
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txncalendarview"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// calendarLabels binds the pure presentation layer to the shared catalog and formatter.
func calendarLabels(pr prefs.Prefs) txncalendarview.Labels {
	return txncalendarview.Labels{T: uistate.T, Ledger: fmtLedgerAmount, Prefs: pr, ScopePrefs: pr}
}

// These adapters retain private call sites and existing component tests while
// keeping their formatting and enable decisions in the native-tested package.
func balanceStr(runBal map[string]money.Money, id string) string {
	text, _ := txncalendarview.Balance(runBal, id)
	return text
}

func balanceTone(runBal map[string]money.Money, id string) string {
	_, tone := txncalendarview.Balance(runBal, id)
	return tone
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

// txnCalendarWidget subscribes to calendar inputs and binds a day pick to the
// ledger's own filter and view atoms. All projection decisions are pure.
func txnCalendarWidget(props txnCalendarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	viewAtom := uistate.UseTxnViewMode()
	filterAtom := uistate.UseTxFilter()
	pr := uistate.UsePrefs().Get()
	lens := uistate.UseMemberLens()
	f := filterAtom.Get()
	labels := calendarLabels(pr)
	labels.ScopePrefs = uistate.LoadPrefs()
	month := useTxnCalendarMonth(props, f, lens, labels, time.Now())
	pickDay := func(dateKey string) {
		selection := labels.PickDay(dateKey, f.From, f.To)
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { *x = selection.Apply(*x) })
		viewAtom.Set(selection.ViewMode)
		uistate.PostNotice(selection.Notice, false)
	}
	return ui.CreateElement(txnCalCalendar, txnCalCalendarProps{
		Month: *month, Headers: labels.Headers(pr.WeekStartWeekday()), OnPick: pickDay,
	})
}

// useTxnCalendarMonth reuses the projection while selection, toolbar disclosure
// or view mode changes. Data, scope, calendar day and localized date labels are
// explicit inputs, so returning to the calendar never retains an obsolete month.
func useTxnCalendarMonth(props txnCalendarProps, filter txnfilter.Criteria, lens []string, labels txncalendarview.Labels, now time.Time) *txncalendarview.Month {
	return ui.UseMemo(func() *txncalendarview.Month {
		month := labels.Build(txncalendarview.Input{
			Now: now, WeekStart: labels.Prefs.WeekStartWeekday(), Filter: filter, MemberLens: lens,
			Shown: props.Shown, Transactions: props.App.Transactions(), Recurring: props.App.Recurring(),
			Accounts: props.App.Accounts(), Settled: props.App.BillMatchPaidOccurrences(),
			Rates: currency.Rates{Base: props.Base, Rates: props.App.Settings().FXRates},
		})
		return &month
	}, revKey(props.App), filter, fmt.Sprintf("%#v", lens), props.Shown, len(props.Shown), props.Base,
		now.Format("2006-01-02"), labels.Prefs.WeekStartWeekday(), labels.Prefs.DateStyle, labels.ScopePrefs.DateStyle, uistate.ActiveLanguage())
}

type txnCalCalendarProps struct {
	Month   txncalendarview.Month
	Headers []txncalendarview.Header
	OnPick  func(string)
}

// txnCalCalendar owns the static tile layout, caption and projection notices.
func txnCalCalendar(props txnCalCalendarProps) ui.Node {
	nav := Div(css.Class("txn-cal-nav", "row"),
		Span(css.Class("txn-cal-month"), Attr("data-testid", "txn-cal-month"), props.Month.Caption),
		Span(css.Class("txn-cal-scroll-hint"), uistate.T("transactions.calScrollHint")))
	if props.Month.Err != nil {
		return uiw.Widget(uiw.WidgetProps{
			ID: "txn-calendar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: Div(css.Class("txn-cal"), nav,
				Div(css.Class("text-dim"), Attr("role", "status"), Attr("data-testid", "txn-cal-fx-error"),
					uistate.T("transactions.calFxMissingRate"))),
		})
	}
	rows := MapKeyed(props.Month.Weeks, func(week txncalendarview.Week) any { return week.Total.SummaryID },
		func(week txncalendarview.Week) ui.Node {
			return ui.CreateElement(txnCalWeekRow, txnCalWeekRowProps{Week: week, OnPick: props.OnPick})
		})
	fxNotice := If(props.Month.RateIssues > 0,
		Div(css.Class("text-dim"), Attr("role", "status"), Attr("data-testid", "txn-cal-fx-warning"),
			uistate.T("transactions.tq3.monthFxMissing", props.Month.RateIssues)))
	return uiw.Widget(uiw.WidgetProps{
		ID: "txn-calendar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("txn-cal"), nav, fxNotice,
			Div(css.Class("txn-cal-viewport"),
				Div(css.Class("txn-cal-weeks"), ui.CreateElement(txnCalWeekHeader, props.Headers), rows)),
			txnCalOverflow(props.Month.Weeks),
			Div(css.Class("txn-cal-legend"),
				Span(css.Class("txn-cal-legend-dots"), Span(css.Class("cal-dot txn-cal-legend-dot"))),
				Span(uistate.T("transactions.calDotsLegend"))),
			If(props.Month.ShowScheduled, Div(css.Class("text-dim"), uistate.T("transactions.calDueFilterNote")))),
	})
}

// txnCalWeekHeader renders the localized weekday columns and total heading.
func txnCalWeekHeader(headers []txncalendarview.Header) ui.Node {
	head := []any{css.Class("txn-cal-week-head")}
	for _, header := range headers {
		head = append(head, Div(css.Class("cal-head"), Attr("role", "columnheader"), Attr("aria-label", header.Aria), header.Text))
	}
	head = append(head, Div(css.Class("cal-head"), uistate.T("transactions.calWeekTotal")))
	return Div(head...)
}

type txnCalWeekRowProps struct {
	Week   txncalendarview.Week
	OnPick func(string)
}

// txnCalWeekRow renders one keyed row; each interactive day owns its click hook.
func txnCalWeekRow(props txnCalWeekRowProps) ui.Node {
	cells := MapKeyed(props.Week.Days, func(day txncalendarview.Day) any { return day.DateKey },
		func(day txncalendarview.Day) ui.Node {
			return ui.CreateElement(txnCalMonthCell, txnCalMonthCellProps{
				DateKey: day.DateKey, Day: day.Day, InMonth: day.InMonth, MonthName: day.MonthName,
				Today: day.Today, NetStr: day.NetStr, NetTone: day.NetTone, MovedStr: day.MovedStr,
				AriaLabel: day.AriaLabel, Count: day.Count, Ghosts: day.Ghosts, Rates: day.Rates, OnPick: props.OnPick,
			})
		})
	return Div(css.Class("txn-cal-week-row"), cells, ui.CreateElement(txnCalWeekTotal, props.Week.Total))
}

type txnCalWeekTotalProps = txncalendarview.WeekInput

// txnCalWeekTotal renders preformatted weekly lines and their complete description.
func txnCalWeekTotal(props txnCalWeekTotalProps) ui.Node {
	summary := calendarLabels(uistate.LoadPrefs()).WeekSummary(props)
	lines := make([]any, 0, len(summary.Lines))
	for _, line := range summary.Lines {
		lines = append(lines, Span(ClassStr(line.Class), line.Text))
	}
	return Div(css.Class("txn-cal-week-total"),
		Attr("data-testid", "txn-cal-week-total"), Attr("aria-describedby", props.SummaryID), Attr("title", summary.Aria),
		Span(css.Class("sr-only"), Attr("id", props.SummaryID), summary.Aria), Fragment(lines...))
}

// txnCalOverflow keeps hidden scheduled items available to touch users without
// making the month grid's day cells grow or changing the day-to-ledger action.
func txnCalOverflow(weeks []txncalendarview.Week) ui.Node {
	labels := calendarLabels(uistate.LoadPrefs())
	rows := make([]any, 0)
	count := 0
	for _, week := range weeks {
		for _, day := range week.Days {
			if !day.InMonth || len(day.Ghosts) < 2 {
				continue
			}
			preview := txnCalPreviewIndex(day.Ghosts)
			items := make([]any, 0, len(day.Ghosts)-1)
			for i, ghost := range day.Ghosts {
				if i == preview {
					continue
				}
				summary := labels.GhostSummary(ghost, day.Rates)
				items = append(items, Div(css.Class("txn-cal-overflow-item"),
					Span(summary.Status), Span(summary.Label)))
				count++
			}
			rows = append(rows, Div(css.Class("txn-cal-overflow-day"),
				Span(css.Class("txn-cal-overflow-date"), uistate.T("transactions.calDayNumber", day.Day)),
				Div(css.Class("txn-cal-overflow-items"), Fragment(items...))))
		}
	}
	if count == 0 {
		return nil
	}
	return Details(css.Class("txn-cal-overflow"), Attr("data-testid", "txn-cal-overflow"),
		Summary(uistate.T("transactions.calOtherDue", count)),
		Div(css.Class("txn-cal-overflow-list"), Fragment(rows...)))
}

func txnCalPreviewIndex(ghosts []txncalendar.Ghost) int {
	for i, ghost := range ghosts {
		if ghost.NotPosted {
			return i
		}
	}
	return 0
}

func scheduledGhosts(ghosts []txncalendar.Ghost, visible bool) []txncalendar.Ghost {
	return txncalendarview.ScheduledGhosts(ghosts, visible)
}

func calNetStr(s txncalendar.DayStat, base string) string {
	return calendarLabels(uistate.LoadPrefs()).Net(s, base)
}

func calMovedStr(amount int64, base string) string {
	return calendarLabels(uistate.LoadPrefs()).Moved(amount, base)
}

func calNetTone(s txncalendar.DayStat) string {
	return txncalendarview.NetTone(s.Net)
}

func calDayAria(day int, cash txncalendar.Cashflow, base string) string {
	return calendarLabels(uistate.LoadPrefs()).DayAria(day, cash, base)
}

func calendarMonthYear(t time.Time) string {
	return calendarLabels(uistate.LoadPrefs()).MonthYear(t)
}

func calendarWeekday(day time.Weekday, full bool) string {
	return calendarLabels(uistate.LoadPrefs()).Weekday(day, full)
}

// txnCalMonthCell renders one day button with its own stable event hooks.
func txnCalMonthCell(props txnCalMonthCellProps) ui.Node {
	summary := calendarLabels(uistate.LoadPrefs()).CellSummary(txncalendarview.Day{
		DateKey: props.DateKey, Day: props.Day, InMonth: props.InMonth, MonthName: props.MonthName,
		Today: props.Today, NetStr: props.NetStr, MovedStr: props.MovedStr, AriaLabel: props.AriaLabel, Count: props.Count,
		Ghosts: props.Ghosts, Rates: props.Rates,
	})
	pick := ui.UseEvent(func() {
		if summary.Pickable && props.OnPick != nil {
			props.OnPick(props.DateKey)
		}
	})
	dots := make([]any, 0, summary.Dots)
	for range summary.Dots {
		dots = append(dots, Span(css.Class("cal-dot")))
	}
	// Keep the calendar's rows a consistent height. Show the most urgent due item
	// and summarize the rest; the full list remains in the button's accessible
	// name without duplicating it in a browser tooltip.
	ghosts := make([]any, 0, 2)
	if len(summary.Ghosts) > 0 {
		preview := txnCalPreviewIndex(props.Ghosts)
		ghosts = append(ghosts, ui.CreateElement(txnCalGhostMarker, summary.Ghosts[preview]))
		if extra := len(summary.Ghosts) - 1; extra > 0 {
			ghosts = append(ghosts, Span(css.Class("txn-cal-more-due"), uistate.T("transactions.calMoreDue", extra)))
		}
	}
	args := []any{ClassStr(summary.Class), Type("button"), Attr("data-date", props.DateKey),
		Attr("data-testid", "txn-cal-day-"+props.DateKey), Attr("aria-label", summary.Aria)}
	click := OnClick(pick)
	if summary.Pickable {
		args = append(args, click)
		if summary.Current {
			args = append(args, Attr("aria-current", "date"))
		}
	} else {
		args = append(args, Disabled(true))
	}
	args = append(args,
		Span(css.Class("cal-day"), summary.DayText),
		If(summary.ShowNet, Span(ClassStr("txn-cal-net "+props.NetTone), props.NetStr)),
		If(summary.ShowMoved, Span(css.Class("txn-cal-moved text-dim"), props.MovedStr)),
		Span(css.Class("txn-cal-dots"), Fragment(dots...)), Fragment(ghosts...))
	return Button(args...)
}

// txnCalGhostMarker renders the existing inline, noninteractive scheduled label.
func txnCalGhostMarker(props txncalendarview.GhostSummary) ui.Node {
	return Span(css.Class(props.Class),
		Span(css.Class("txn-cal-ghost-status"), props.Status),
		Span(css.Class("txn-cal-ghost-label"), props.Label))
}

func calTransactionDotCount(count int) int {
	return txncalendarview.DotCount(count)
}
