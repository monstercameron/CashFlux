// SPDX-License-Identifier: MIT

// Package dateutil provides date and period helpers for CashFlux: parsing the
// canonical date format, month/week/fiscal-month ranges, and day math. These
// drive budget periods, forecasts, and reporting.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package dateutil

import (
	"fmt"
	"time"
)

// Layout is the canonical date format used throughout CashFlux (ISO 8601 date).
const Layout = "2006-01-02"

// ParseDate parses a "YYYY-MM-DD" string in UTC.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(Layout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("dateutil: parse %q: %w", s, err)
	}
	return t, nil
}

// FormatDate renders a time as "YYYY-MM-DD".
func FormatDate(t time.Time) string { return t.Format(Layout) }

// CashFlux dates are timezone-free calendar dates stored at UTC midnight:
// ParseDate parses "YYYY-MM-DD" in UTC, and transaction/account dates are kept
// that way. Period boundaries must therefore also be UTC-midnight calendar
// dates, or a first-of-period transaction dated 00:00Z is dropped on any
// machine in a timezone behind UTC (the local month-start lands *after* it).
// The boundary builders below take the calendar date from t (in t's own
// location, i.e. the user's wall calendar) but emit the boundary at UTC
// midnight so it compares cleanly against stored dates (C1).

// midnight returns t's calendar day at 00:00 UTC.
func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DayStart returns t's calendar day at 00:00 UTC — the start-of-day boundary
// used for date-only bucketing (e.g. per-day balance series).
func DayStart(t time.Time) time.Time { return midnight(t) }

// MonthStart returns the first day of t's month at 00:00 UTC.
func MonthStart(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

// AddMonths returns t shifted by n calendar months (n may be negative).
func AddMonths(t time.Time, n int) time.Time { return t.AddDate(0, n, 0) }

// MonthRange returns the half-open range [start, end) covering t's month.
func MonthRange(t time.Time) (start, end time.Time) {
	start = MonthStart(t)
	end = AddMonths(start, 1)
	return start, end
}

// WeekStart returns the start of the week (00:00) containing t, where the week
// begins on weekStart (e.g. time.Monday or time.Sunday).
func WeekStart(t time.Time, weekStart time.Weekday) time.Time {
	offset := (int(t.Weekday()) - int(weekStart) + 7) % 7
	return midnight(t).AddDate(0, 0, -offset)
}

// FiscalMonthRange returns the half-open range [start, end) for the fiscal month
// containing t, where each fiscal month begins on startDay (clamped to 1..28).
func FiscalMonthRange(t time.Time, startDay int) (start, end time.Time) {
	if startDay < 1 {
		startDay = 1
	}
	if startDay > 28 {
		startDay = 28
	}
	y, m, _ := t.Date()
	anchor := time.Date(y, m, startDay, 0, 0, 0, 0, time.UTC)
	if midnight(t).Before(anchor) {
		anchor = AddMonths(anchor, -1)
	}
	return anchor, AddMonths(anchor, 1)
}

// InRange reports whether t is within the half-open range [start, end).
func InRange(t, start, end time.Time) bool {
	return !t.Before(start) && t.Before(end)
}

// DaysBetween returns the whole-day count from a to b (b − a), ignoring time of
// day and time zones (both reduced to their calendar date).
func DaysBetween(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	a0 := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	b0 := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int(b0.Sub(a0).Hours()) / 24
}

// NextMonthlyDue returns the next occurrence of a monthly due-day on or after
// the day containing `now`, at 00:00 UTC (the canonical calendar-date basis;
// see the boundary-builder note above). The day is clamped to 28 so it stays
// valid in every month, including February.
func NextMonthlyDue(now time.Time, day int) time.Time {
	if day > 28 {
		day = 28
	}
	if day < 1 {
		day = 1
	}
	y, m, d := now.Date()
	due := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if due.Before(today) {
		due = AddMonths(due, 1)
	}
	return due
}
