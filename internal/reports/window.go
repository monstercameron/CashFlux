// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// This file resolves the window a report was ASKED for — "last quarter",
// "2026-03-01 to 2026-03-31" — into the half-open bounds every report function
// takes, plus the equivalent preceding window the "vs last period" columns
// compare against.
//
// It is pure, and separate from the assistant's tool layer, because this is the
// part that is easy to get quietly wrong: an inclusive end date treated as
// exclusive silently drops the last day of every window, and no figure on
// screen would look odd enough to notice.

// AskedWindow is a resolved reporting window: [Start, End) with the preceding
// window of the same shape, and a label to caption an answer with. A zero Start
// means "everything up to End" — the all-time case, which has no predecessor.
type AskedWindow struct {
	Start, End         time.Time
	PrevStart, PrevEnd time.Time
	Label              string
}

// HasPrevious reports whether a comparison window was resolved.
func (w AskedWindow) HasPrevious() bool { return !w.PrevStart.IsZero() }

// ResolveWindow turns a named period — or an explicit from/to pair — into an
// AskedWindow relative to now.
//
// Explicit dates always win over the name, and `to` is INCLUSIVE, as a person
// means it: "to the 31st" includes the 31st. Either date may be omitted, which
// leaves that end open (from the beginning, or through today). An unrecognised
// name resolves to the current month rather than to nothing, so a mistyped
// period returns a real answer the caller can see is not the one they wanted.
func ResolveWindow(preset, from, to string, now time.Time) AskedWindow {
	fromT, fromErr := dateutil.ParseDate(strings.TrimSpace(from))
	toT, toErr := dateutil.ParseDate(strings.TrimSpace(to))
	if fromErr == nil || toErr == nil {
		w := AskedWindow{End: dateutil.DayStart(now).AddDate(0, 0, 1)}
		if fromErr == nil {
			w.Start = dateutil.DayStart(fromT)
		}
		if toErr == nil {
			w.End = dateutil.DayStart(toT).AddDate(0, 0, 1)
		}
		if !w.Start.IsZero() {
			span := w.End.Sub(w.Start)
			w.PrevStart, w.PrevEnd = w.Start.Add(-span), w.Start
		}
		w.Label = explicitWindowLabel(w.Start, w.End)
		return w
	}

	monthStart, monthEnd := dateutil.MonthRange(now)
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "last_month":
		s, e := dateutil.MonthRange(dateutil.AddMonths(now, -1))
		ps, pe := dateutil.MonthRange(dateutil.AddMonths(now, -2))
		return AskedWindow{Start: s, End: e, PrevStart: ps, PrevEnd: pe, Label: s.Format("January 2006")}
	case "this_quarter":
		s := QuarterStart(now)
		return AskedWindow{Start: s, End: dateutil.AddMonths(s, 3),
			PrevStart: dateutil.AddMonths(s, -3), PrevEnd: s, Label: QuarterLabel(s)}
	case "last_quarter":
		s := dateutil.AddMonths(QuarterStart(now), -3)
		return AskedWindow{Start: s, End: dateutil.AddMonths(s, 3),
			PrevStart: dateutil.AddMonths(s, -3), PrevEnd: s, Label: QuarterLabel(s)}
	case "this_year":
		s := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return AskedWindow{Start: s, End: s.AddDate(1, 0, 0),
			PrevStart: s.AddDate(-1, 0, 0), PrevEnd: s, Label: s.Format("2006")}
	case "last_year":
		s := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
		return AskedWindow{Start: s, End: s.AddDate(1, 0, 0),
			PrevStart: s.AddDate(-1, 0, 0), PrevEnd: s, Label: s.Format("2006")}
	case "year_to_date", "ytd":
		s := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		return AskedWindow{Start: s, End: dateutil.DayStart(now).AddDate(0, 0, 1),
			PrevStart: s.AddDate(-1, 0, 0), PrevEnd: s, Label: fmt.Sprintf("%d so far", now.Year())}
	case "last_12_months":
		s := dateutil.AddMonths(monthStart, -11)
		return AskedWindow{Start: s, End: monthEnd,
			PrevStart: dateutil.AddMonths(s, -12), PrevEnd: s,
			Label: s.Format("Jan 2006") + " – " + monthStart.Format("Jan 2006")}
	case "all":
		// All time has no predecessor to compare against, and saying so is the
		// point: a "vs last period" column over an open-ended window would be
		// comparing against nothing.
		return AskedWindow{End: dateutil.DayStart(now).AddDate(0, 0, 1), Label: "all time"}
	default:
		ps, pe := dateutil.MonthRange(dateutil.AddMonths(now, -1))
		return AskedWindow{Start: monthStart, End: monthEnd, PrevStart: ps, PrevEnd: pe,
			Label: monthStart.Format("January 2006")}
	}
}

// QuarterStart is the first day of the calendar quarter containing t.
func QuarterStart(t time.Time) time.Time {
	m := ((int(t.Month()) - 1) / 3) * 3
	return time.Date(t.Year(), time.Month(m+1), 1, 0, 0, 0, 0, t.Location())
}

// QuarterLabel names a quarter from its first day ("Q3 2026").
func QuarterLabel(start time.Time) string {
	return fmt.Sprintf("Q%d %d", (int(start.Month())-1)/3+1, start.Year())
}

// explicitWindowLabel captions a from/to window with the INCLUSIVE last day, so
// the caption matches what the user asked for rather than the exclusive bound
// the maths uses.
func explicitWindowLabel(start, end time.Time) string {
	last := end.AddDate(0, 0, -1)
	if start.IsZero() {
		return "everything up to " + dateutil.FormatDate(last)
	}
	return dateutil.FormatDate(start) + " – " + dateutil.FormatDate(last)
}

// ParseMonth reads a month the way a person writes one: "2026-03", "March
// 2026", "Mar 2026", a full date, or a bare month name (taken as this year).
// It returns the first day of that month.
func ParseMonth(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01", "January 2006", "Jan 2006", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return dateutil.MonthStart(t), true
		}
	}
	for _, layout := range []string{"January", "Jan"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(now.Year(), t.Month(), 1, 0, 0, 0, 0, now.Location()), true
		}
	}
	return time.Time{}, false
}
