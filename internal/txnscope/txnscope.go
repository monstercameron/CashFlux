// SPDX-License-Identifier: MIT

// Package txnscope answers one question for the transactions ledger: what period
// am I looking at?
//
// The ledger already stored its date scope as the filter's From/To pair, which can
// hold any range — a month, a week from the filter panel, a single day picked out
// of the calendar, or nothing at all. The page ALSO showed the top bar's reporting
// period pill, which stores a snapped period.Window and cannot represent an
// arbitrary range, and a calendar whose visible month came from a third piece of
// state anchored on time.Now(). Three sources, no agreement: the pill could read
// "Jul 2026" over August rows, and stepping it changed neither the rows nor the
// totals (C560).
//
// This package makes From/To the single source. It is the only one of the three
// that can express everything the ledger actually supports, so the label, the
// calendar's anchor and the rows are all derived from it and cannot drift. The
// month stepper writes it; the filter panel writes it; a calendar day-click writes
// it — and every one of them moves the same state.
//
// Pure (no syscall/js) and unit-tested on native Go.
package txnscope

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// Kind classifies what the ledger's date range currently represents.
type Kind int

const (
	// All means no date bounds: the ledger shows every transaction.
	All Kind = iota
	// Month means the bounds are exactly one calendar month, so the scope can be
	// labelled by that month's name and stepped a month at a time.
	Month
	// Day means the bounds are a single date — what a calendar day-click leaves.
	Day
	// Custom means an arbitrary range (a hand-typed From/To, or a preset that does
	// not align to a month). It is labelled as the range it is, never as a month it
	// only partly covers.
	Custom
)

// Scope is the ledger's date scope, derived from the filter's From/To strings.
// Anchor is the month the calendar should show — for every Kind, including All,
// so the calendar always has a month to draw.
type Scope struct {
	Kind   Kind
	From   time.Time
	To     time.Time // inclusive, as the filter stores it
	Anchor time.Time // first day of the month the scope sits in
}

// Of derives the scope from the filter's From/To strings (both "YYYY-MM-DD" or
// empty), with now supplying the anchor when there are no bounds to take one from.
//
// A half-open range (one bound set) is Custom: it is a real, expressible ledger
// scope, and calling it a month would be the exact class of lie this package
// exists to prevent.
func Of(from, to string, now time.Time) Scope {
	f, fOK := parse(from)
	t, tOK := parse(to)

	switch {
	case !fOK && !tOK:
		return Scope{Kind: All, Anchor: dateutil.MonthStart(now)}
	case fOK && tOK && f.Equal(t):
		return Scope{Kind: Day, From: f, To: t, Anchor: dateutil.MonthStart(f)}
	case fOK && tOK && isWholeMonth(f, t):
		return Scope{Kind: Month, From: f, To: t, Anchor: f}
	}

	// Custom. Anchor on whichever bound exists, preferring the start.
	anchor := now
	if fOK {
		anchor = f
	} else if tOK {
		anchor = t
	}
	return Scope{Kind: Custom, From: f, To: t, Anchor: dateutil.MonthStart(anchor)}
}

// MonthBounds returns the From/To filter strings for the whole calendar month
// containing t. To is the month's LAST day, matching the filter's inclusive
// upper bound.
func MonthBounds(t time.Time) (from, to string) {
	start := dateutil.MonthStart(t)
	end := dateutil.AddMonths(start, 1).AddDate(0, 0, -1)
	return dateutil.FormatDate(start), dateutil.FormatDate(end)
}

// Step returns the From/To for the month delta months away from the scope's
// anchor. Stepping from ANY scope — a single day, a custom range, or all dates —
// lands on a whole month, because that is what the stepper's label promises. From
// All it steps relative to the anchor month, so "‹" from All dates shows the month
// before the current one rather than doing nothing.
func (s Scope) Step(delta int) (from, to string) {
	return MonthBounds(dateutil.AddMonths(s.Anchor, delta))
}

// IsCurrentMonth reports whether the scope is exactly the calendar month
// containing now — what the "This month" control resets to, so the control can
// show itself as already satisfied.
func (s Scope) IsCurrentMonth(now time.Time) bool {
	return s.Kind == Month && s.Anchor.Equal(dateutil.MonthStart(now))
}

// isWholeMonth reports whether [from, to] is exactly one calendar month: from is
// the first of a month and to is that same month's last day.
func isWholeMonth(from, to time.Time) bool {
	start := dateutil.MonthStart(from)
	if !from.Equal(start) {
		return false
	}
	last := dateutil.AddMonths(start, 1).AddDate(0, 0, -1)
	return to.Equal(last)
}

// parse reads a filter date string, reporting whether it held a usable date.
func parse(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := dateutil.ParseDate(s)
	if err != nil {
		return time.Time{}, false
	}
	return dateutil.DayStart(t), true
}
