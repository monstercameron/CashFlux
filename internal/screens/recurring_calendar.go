// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// rhyCalendarProps carries the occurrences the agenda already computed, so the
// calendar and the list cannot disagree about what is due when.
type rhyCalendarProps struct {
	Occurrences []recurOccurrence
}

// rhyCalendarSection renders the month view of bills and recurring cash flows on
// the shared `uiw.Calendar` primitive (LF-7).
//
// The page had a calendar already — a hand-rolled `cal-grid` in `bills_screen.go`
// that the unified surface no longer routes to. Rebuilding it on the primitive is
// the point of the ticket: one calendar means one set of keyboard behaviour, one
// week-start rule, one set of cell test ids, and a month-paging control that
// works the same here as on the to-do board.
//
// It reads the SAME occurrence list the agenda renders. Recomputing from
// `bills.UpcomingAll` here would be the classic two-screens-disagree shape — the
// list would show a payment the grid did not, and neither would be wrong on its
// own terms.
func rhyCalendarSection(props rhyCalendarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	open := ui.UseState(false)
	// offset is months from the current one, so paging survives a re-render
	// without holding a date that goes stale at midnight.
	offset := ui.UseState(0)

	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	prev := func() { offset.Set(offset.Get() - 1) }
	next := func() { offset.Set(offset.Get() + 1) }
	today := func() { offset.Set(0) }
	onToday := ui.UseEvent(Prevent(today))

	if !open.Get() {
		return Div(css.Class("rhy-section rhy-cal-entry"),
			Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "rhy-cal-open"),
				Attr("aria-expanded", "false"), OnClick(toggle), uistate.T("rhyCal.open")))
	}

	now := time.Now()
	month := dateutil.AddMonths(dateutil.MonthStart(now), offset.Get())
	prefs := uistate.LoadPrefs()

	// Bucket by day. A day with three payments is one cell with a count, not three
	// cells — the grid answers "what lands when", and the agenda below answers
	// "what exactly".
	byDay := map[string][]recurOccurrence{}
	for _, o := range props.Occurrences {
		k := o.Date.Format("2006-01-02")
		byDay[k] = append(byDay[k], o)
	}

	base := "USD"
	if app := appstate.Default; app != nil {
		if c := app.Settings().BaseCurrency; c != "" {
			base = c
		}
	}

	dayContent := func(day time.Time, inMonth bool) ui.Node {
		if !inMonth {
			return Fragment()
		}
		occs := byDay[day.Format("2006-01-02")]
		if len(occs) == 0 {
			return Fragment()
		}
		var due int64
		var paid, overdue int
		for _, o := range occs {
			due += absMinor(o.R.Amount.Amount)
			if o.Paid {
				paid++
			}
			if o.Overdue && !o.Paid {
				overdue++
			}
		}
		// The marker carries state, not just presence: an overdue day and a settled
		// day are opposite readings, and a single neutral dot for both is the kind
		// of "information" that costs a click to interpret.
		cls := "rhy-cal-dot"
		switch {
		case overdue > 0:
			cls += " is-overdue"
		case paid == len(occs):
			cls += " is-paid"
		}
		label := fmtMoney(money.New(due, base))
		if len(occs) > 1 {
			label = strconv.Itoa(len(occs)) + " · " + label
		}
		return Span(ClassStr(cls), Attr("data-testid", "rhy-cal-mark-"+day.Format("2006-01-02")),
			Attr("title", rhyCalDayTitle(occs, base)), label)
	}

	return Div(css.Class("rhy-section rhy-cal"), Attr("data-testid", "rhy-cal"),
		Div(css.Class("rhy-cal-head"),
			Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "rhy-cal-close"),
				Attr("aria-expanded", "true"), OnClick(toggle), uistate.T("rhyCal.close")),
			If(offset.Get() != 0, Button(css.Class("btn-link", tw.Text12), Type("button"),
				Attr("data-testid", "rhy-cal-today"), OnClick(onToday), uistate.T("rhyCal.today"))),
		),
		uiw.Calendar(uiw.CalendarProps{
			Month:        month,
			WeekStart:    prefs.WeekStartWeekday(),
			Today:        now,
			DayContent:   dayContent,
			OnPrevMonth:  prev,
			OnNextMonth:  next,
			Label:        uistate.T("rhyCal.label"),
			TestIDPrefix: "rhy-cal",
		}),
		P(css.Class("muted", tw.Text12), uistate.T("rhyCal.legend")),
	)
}

// rhyCalDayTitle names what lands on a day, for the cell's tooltip. Amounts are
// included because "3 payments" without the total is a number that raises a
// question rather than answering one.
func rhyCalDayTitle(occs []recurOccurrence, base string) string {
	out := ""
	for i, o := range occs {
		if i > 0 {
			out += " · "
		}
		out += o.R.Label + " " + fmtMoney(money.New(absMinor(o.R.Amount.Amount), base))
		if o.Paid {
			out += " ✓"
		}
	}
	return out
}
