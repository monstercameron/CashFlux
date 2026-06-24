// SPDX-License-Identifier: MIT

// Package period models the dashboard's time-resolution control: a Resolution
// (week, month, or quarter) plus anchor math to step, label, and turn a
// from/to anchor pair into a concrete half-open date range for reporting.
//
// Pure Go, no platform dependencies; unit-tested on native Go. The UI layer
// holds the selected resolution and anchors in state and calls into here; all
// date arithmetic lives here and in internal/dateutil.
package period

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// Resolution is the granularity of the dashboard period.
type Resolution string

// The supported resolutions.
const (
	Week    Resolution = "week"
	Month   Resolution = "month"
	Quarter Resolution = "quarter"
	Year    Resolution = "year"
)

// Valid reports whether r is a known resolution.
func (r Resolution) Valid() bool {
	switch r {
	case Week, Month, Quarter, Year:
		return true
	default:
		return false
	}
}

// String returns the resolution's lowercase token.
func (r Resolution) String() string { return string(r) }

// Truncate snaps t to the start of the unit containing it for the given
// resolution (week start at weekStart, month start, or quarter start).
func Truncate(r Resolution, t time.Time, weekStart time.Weekday) time.Time {
	switch r {
	case Week:
		return dateutil.WeekStart(t, weekStart)
	case Quarter:
		return quarterStart(t)
	case Year:
		return yearStart(t)
	default: // Month
		return dateutil.MonthStart(t)
	}
}

// Step moves an anchor by delta whole units of the resolution (delta may be
// negative). The anchor is assumed to already sit at a unit boundary.
func Step(r Resolution, t time.Time, delta int) time.Time {
	switch r {
	case Week:
		return t.AddDate(0, 0, 7*delta)
	case Quarter:
		return dateutil.AddMonths(t, 3*delta)
	case Year:
		return dateutil.AddMonths(t, 12*delta)
	default: // Month
		return dateutil.AddMonths(t, delta)
	}
}

// Label renders a unit anchor for display, e.g. "Jun 2 – Jun 8" (week),
// "Jun 2026" (month), or "Q3 2026" (quarter). The anchor is snapped to its unit
// start first so callers may pass any day within the unit.
func Label(r Resolution, t time.Time, weekStart time.Weekday) string {
	switch r {
	case Week:
		start := dateutil.WeekStart(t, weekStart)
		end := start.AddDate(0, 0, 6)
		return start.Format("Jan 2") + " – " + end.Format("Jan 2")
	case Quarter:
		s := quarterStart(t)
		return fmt.Sprintf("Q%d %d", int(s.Month()-1)/3+1, s.Year())
	case Year:
		return fmt.Sprintf("%d", yearStart(t).Year())
	default: // Month
		return dateutil.MonthStart(t).Format("Jan 2006")
	}
}

// Range returns the half-open range [start, end) spanning from the from-anchor's
// unit through the end of the to-anchor's unit. If to precedes from the range
// collapses to just the from unit (callers should clamp from <= to themselves).
func Range(r Resolution, from, to time.Time, weekStart time.Weekday) (start, end time.Time) {
	start = Truncate(r, from, weekStart)
	toStart := Truncate(r, to, weekStart)
	if toStart.Before(start) {
		toStart = start
	}
	end = Step(r, toStart, 1)
	return start, end
}

// yearStart returns the first day of the calendar year containing t (UTC-midnight
// boundary, matching the UTC-dated transactions it's compared against).
func yearStart(t time.Time) time.Time {
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
}

// quarterStart returns the first day of the calendar quarter containing t.
func quarterStart(t time.Time) time.Time {
	q := (int(t.Month()) - 1) / 3 // 0..3
	// UTC-midnight calendar boundary, matching dateutil and the UTC-dated
	// transactions it's compared against (C1).
	return time.Date(t.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, time.UTC)
}
