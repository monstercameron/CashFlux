// SPDX-License-Identifier: MIT

// Package calendargrid computes the month-grid geometry for a reusable calendar
// primitive: the weeks-of-days covering a month (padded to whole weeks from a
// chosen week-start) and the weekday header order. It is pure Go — no
// syscall/js, no time.Now() — so a caller passes both the anchor month and the
// "today" date explicitly, keeping the grid deterministic and unit-testable.
package calendargrid

import "time"

// Day is one cell in the month grid: its calendar date (normalized to local
// midnight), whether it falls inside the anchored month (padding days from the
// neighbouring months are false), and whether it is the caller-supplied "today".
type Day struct {
	Date    time.Time
	InMonth bool
	IsToday bool
}

// midnight returns t at 00:00 in t's own location.
func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// sameDate reports whether a and b are the same calendar day (year/month/day),
// ignoring time-of-day and location differences.
func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// Month returns the calendar weeks (each a []Day of length 7) covering the month
// that `anchor` falls in, padded to whole weeks starting on weekStart. Dates are
// normalized to local midnight in anchor's location. IsToday is set when a day
// equals `today` (compared by calendar date). A month spans 4, 5, or 6 weeks
// depending on its length and starting weekday.
func Month(anchor time.Time, weekStart time.Weekday, today time.Time) [][]Day {
	y, m, _ := anchor.Date()
	loc := anchor.Location()
	first := time.Date(y, m, 1, 0, 0, 0, 0, loc)

	// Lead-in padding: how many days back the grid starts so the first row begins
	// on weekStart.
	offset := (int(first.Weekday()) - int(weekStart) + 7) % 7
	gridStart := first.AddDate(0, 0, -offset)

	// Whole weeks needed to cover offset + the month's own days, rounded up.
	daysInMonth := first.AddDate(0, 1, -1).Day()
	weeks := (offset + daysInMonth + 6) / 7

	out := make([][]Day, 0, weeks)
	cur := gridStart
	for w := 0; w < weeks; w++ {
		week := make([]Day, 7)
		for i := 0; i < 7; i++ {
			d := midnight(cur)
			dy, dm, _ := d.Date()
			week[i] = Day{
				Date:    d,
				InMonth: dm == m && dy == y,
				IsToday: sameDate(d, today),
			}
			cur = cur.AddDate(0, 0, 1)
		}
		out = append(out, week)
	}
	return out
}

// WeekdayOrder returns the seven weekdays in header order for a given week-start
// (e.g. [Sun..Sat] for weekStart == time.Sunday, [Mon..Sun] for time.Monday).
func WeekdayOrder(weekStart time.Weekday) []time.Weekday {
	out := make([]time.Weekday, 7)
	for i := 0; i < 7; i++ {
		out[i] = time.Weekday((int(weekStart) + i) % 7)
	}
	return out
}
