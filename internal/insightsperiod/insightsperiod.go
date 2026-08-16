// SPDX-License-Identifier: MIT

// Package insightsperiod resolves the Insights briefing's time range (G2-C8).
//
// The briefing was fixed to month-to-date, which answers "how is this month
// going?" and nothing else. A household asking "was last month unusual?" or "what
// has the year looked like?" had no way to ask it, and the tab called Insights
// could only ever show one insight.
//
// The comparison is the part worth being careful about, and it follows one rule:
// a window is compared against the time immediately before it, matched to its own
// kind. A PARTIAL window matches day-for-day — seventeen days into August is
// compared with the first seventeen days of July, not with the whole of July,
// because comparing a partial period against a complete one is the easiest way to
// make a normal month look like a collapse. A COMPLETE calendar month is compared
// with the previous calendar month, even though months are unequal lengths, because
// "31 days before July" lands in the middle of May and nobody reading "vs last
// month" means that.
//
// Pure: no syscall/js, no appstate. Table-tested on native Go.
package insightsperiod

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// Period is a selectable range for the briefing.
type Period string

const (
	// ThisMonth is the month so far — the default, and the only window the
	// briefing used to have.
	ThisMonth Period = "this-month"
	// LastMonth is the previous calendar month, complete.
	LastMonth Period = "last-month"
	// Last3Months is the trailing three complete months plus the current one.
	Last3Months Period = "last-3-months"
	// Last12Months is the trailing year.
	Last12Months Period = "last-12-months"
)

// All returns the selectable periods in the order they should be offered:
// shortest first, because the shortest is the one asked about most.
func All() []Period { return []Period{ThisMonth, LastMonth, Last3Months, Last12Months} }

// Valid reports whether p is one of the known periods.
func Valid(p Period) bool {
	for _, known := range All() {
		if p == known {
			return true
		}
	}
	return false
}

// Resolve turns a chosen period into the window it covers and the window it is
// compared against. An unknown period resolves as ThisMonth rather than as an
// empty range, so a stale stored value degrades to the default instead of showing
// a briefing about no time at all.
//
// Both windows are half-open [start, end).
func Resolve(p Period, now time.Time) Range {
	switch p {
	case LastMonth:
		// A complete calendar month is compared with the previous CALENDAR month,
		// not with "the same number of days before". Months are unequal lengths, and
		// shifting a 31-day window back by 31 days lands mid-May rather than on
		// June — an off-by-a-day-and-a-half comparison that no reader would expect.
		start := dateutil.AddMonths(dateutil.MonthStart(now), -1)
		end := dateutil.MonthStart(now)
		return Range{
			Period: p, Start: start, End: end,
			PriorStart: dateutil.AddMonths(start, -1), PriorEnd: start,
		}
	case Last3Months:
		start := dateutil.AddMonths(dateutil.MonthStart(now), -2)
		end := dayAfter(now)
		return withPriorSpan(p, start, end)
	case Last12Months:
		start := dateutil.AddMonths(dateutil.MonthStart(now), -11)
		end := dayAfter(now)
		return withPriorSpan(p, start, end)
	default:
		// This month so far. The end is tomorrow so today's transactions count;
		// a window ending at midnight this morning silently omits everything
		// bought today, which is the spending someone is most likely checking.
		start := dateutil.MonthStart(now)
		return withPriorSpan(ThisMonth, start, dayAfter(now))
	}
}

// Range is a resolved window and the equal-length window before it.
type Range struct {
	// Period is the period this range came from.
	Period Period
	// Start and End bound the window, half-open.
	Start, End time.Time
	// PriorStart and PriorEnd bound the immediately preceding window of the same
	// length — the honest comparison for a period that may be partial.
	PriorStart, PriorEnd time.Time
}

// Months reports how many calendar months the window touches, for callers that
// size a trend chart from the selection. It counts months rather than dividing by
// an average length, because a 28-day February and a 31-day August are both one
// month to anyone reading a chart.
func (r Range) Months() int {
	// End is exclusive, so the last instant inside the window is what counts:
	// a window ending at midnight on 1 August covers July, not August.
	last := r.End.Add(-time.Nanosecond)
	if last.Before(r.Start) {
		return 1
	}
	months := (last.Year()-r.Start.Year())*12 + int(last.Month()) - int(r.Start.Month()) + 1
	if months < 1 {
		return 1
	}
	return months
}

// withPriorSpan pairs a window with the equal-length window immediately before it.
func withPriorSpan(p Period, start, end time.Time) Range {
	span := end.Sub(start)
	return Range{
		Period:     p,
		Start:      start,
		End:        end,
		PriorStart: start.Add(-span),
		PriorEnd:   start,
	}
}

// dayAfter returns midnight at the start of the day following t, so a half-open
// window ending there includes everything dated today.
func dayAfter(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return d.AddDate(0, 0, 1)
}

// LabelKey returns the i18n key for a period's label, so the copy lives in the
// catalog with the rest of the UI's words rather than in this package.
func LabelKey(p Period) string {
	switch p {
	case LastMonth:
		return "assistant.periodLastMonth"
	case Last3Months:
		return "assistant.periodLast3"
	case Last12Months:
		return "assistant.periodLast12"
	default:
		return "assistant.periodThisMonth"
	}
}

// ComparisonKey returns the i18n key for how a period's comparison should be
// described. A partial window is compared against the same PART of the previous
// one, and saying so is the difference between a figure that can be trusted and
// one that quietly flatters.
func ComparisonKey(p Period) string {
	switch p {
	case LastMonth:
		return "assistant.comparePrevMonth"
	case Last3Months, Last12Months:
		return "assistant.comparePrevSpan"
	default:
		return "assistant.comparePace"
	}
}
