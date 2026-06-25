// SPDX-License-Identifier: MIT

// Package runway bridges the household's recurring cash flows into the pure
// cashflow projection engine.
package runway

import "time"

// NextPaydayHorizon returns the number of days from `from` (inclusive) to the
// next occurrence of the given day-of-month (payCycleDay). "Next" means the
// first calendar day on or after `from` whose day-of-month equals payCycleDay.
//
// Short-month clamping: if payCycleDay exceeds the number of days in the target
// month it is clamped to the last day of that month (e.g. day 31 in February
// maps to Feb 28/29).
//
// Special cases:
//   - payCycleDay <= 0: the caller has not configured a pay day; return fallbackDays.
//   - Result minimum: 1. Even if payCycleDay matches today, the returned horizon
//     is at least 1 so callers always get a positive window to project into.
func NextPaydayHorizon(from time.Time, payCycleDay int, fallbackDays int) int {
	if payCycleDay <= 0 {
		return fallbackDays
	}

	// Normalize `from` to midnight UTC so all arithmetic is purely calendar-based.
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)

	// Try the target day in the current month first, then the next month.
	for _, candidate := range []func() time.Time{
		func() time.Time { return clampedDay(from.Year(), from.Month(), payCycleDay) },
		func() time.Time { return clampedDay(from.Year(), from.Month()+1, payCycleDay) },
	} {
		t := candidate()
		days := int(t.Sub(from).Hours()) / 24
		if days >= 0 {
			// Enforce a minimum of 1 day.
			if days < 1 {
				days = 1
			}
			return days
		}
	}

	// Unreachable: next-month candidate is always >= 28 days away.
	return fallbackDays
}

// clampedDay returns midnight UTC for the given year/month and day-of-month,
// clamping day to the last day of the month when it overshoots.
func clampedDay(year int, month time.Month, day int) time.Time {
	// time.Date handles month overflow correctly (e.g. month=13 → January next year).
	// To find the last day of the month: day 0 of the following month.
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
