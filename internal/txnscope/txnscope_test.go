// SPDX-License-Identifier: MIT

package txnscope

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// now is deliberately mid-August, the month the C560 report was filed against —
// the pill read "Jul 2026" while the rows and the calendar were in August.
var now = day(2026, time.August, 16)

func TestOfClassifiesTheLedgerRange(t *testing.T) {
	tests := []struct {
		name       string
		from, to   string
		wantKind   Kind
		wantAnchor time.Time
	}{
		{"no bounds is all dates, anchored on today's month", "", "", All, day(2026, time.August, 1)},
		{"a whole calendar month", "2026-08-01", "2026-08-31", Month, day(2026, time.August, 1)},
		{"a whole February in a leap year", "2024-02-01", "2024-02-29", Month, day(2024, time.February, 1)},
		{"a single day, as a calendar click leaves it", "2026-08-14", "2026-08-14", Day, day(2026, time.August, 1)},
		// The near-misses. Each is a range a user can really produce, and calling any
		// of them "August" would put a wrong month over right rows.
		{"a month missing its last day is custom", "2026-08-01", "2026-08-30", Custom, day(2026, time.August, 1)},
		{"a month starting on the 2nd is custom", "2026-08-02", "2026-08-31", Custom, day(2026, time.August, 1)},
		{"February to a 31st is custom", "2024-02-01", "2024-02-31", Custom, day(2024, time.February, 1)},
		{"a multi-month range is custom", "2026-06-01", "2026-08-31", Custom, day(2026, time.June, 1)},
		{"only a start bound is custom", "2026-05-04", "", Custom, day(2026, time.May, 1)},
		{"only an end bound is custom, anchored on it", "", "2026-05-04", Custom, day(2026, time.May, 1)},
		{"an unparseable bound is ignored", "not-a-date", "", All, day(2026, time.August, 1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Of(tc.from, tc.to, now)
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tc.wantKind)
			}
			if !got.Anchor.Equal(tc.wantAnchor) {
				t.Errorf("Anchor = %v, want %v", got.Anchor, tc.wantAnchor)
			}
		})
	}
}

func TestMonthBoundsCoverTheWholeMonth(t *testing.T) {
	tests := []struct {
		name     string
		in       time.Time
		from, to string
	}{
		{"a 31-day month", day(2026, time.August, 16), "2026-08-01", "2026-08-31"},
		{"a 30-day month", day(2026, time.September, 1), "2026-09-01", "2026-09-30"},
		{"a leap February", day(2024, time.February, 12), "2024-02-01", "2024-02-29"},
		{"a common February", day(2026, time.February, 12), "2026-02-01", "2026-02-28"},
		{"December, so the roll crosses a year", day(2026, time.December, 31), "2026-12-01", "2026-12-31"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from, to := MonthBounds(tc.in)
			if from != tc.from || to != tc.to {
				t.Errorf("MonthBounds() = %q..%q, want %q..%q", from, to, tc.from, tc.to)
			}
		})
	}
}

// Whatever the current scope is, stepping lands on a whole month — that is what
// makes the round trip Of(Step(...)) stable, which is what keeps the stepper's
// label and the rows it produces in agreement.
func TestStepAlwaysLandsOnAWholeMonth(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		delta    int
		wantFrom string
		wantTo   string
	}{
		{"back a month from a month", "2026-08-01", "2026-08-31", -1, "2026-07-01", "2026-07-31"},
		{"forward a month from a month", "2026-08-01", "2026-08-31", 1, "2026-09-01", "2026-09-30"},
		{"back across a year boundary", "2026-01-01", "2026-01-31", -1, "2025-12-01", "2025-12-31"},
		{"from a single day, to that day's neighbouring month", "2026-08-14", "2026-08-14", -1, "2026-07-01", "2026-07-31"},
		{"from a custom range, relative to its start", "2026-06-15", "2026-08-02", 1, "2026-07-01", "2026-07-31"},
		{"from all dates, relative to today", "", "", -1, "2026-07-01", "2026-07-31"},
		{"a zero step normalizes the scope to its month", "2026-08-14", "2026-08-14", 0, "2026-08-01", "2026-08-31"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from, to := Of(tc.from, tc.to, now).Step(tc.delta)
			if from != tc.wantFrom || to != tc.wantTo {
				t.Errorf("Step(%d) = %q..%q, want %q..%q", tc.delta, from, to, tc.wantFrom, tc.wantTo)
			}
			if k := Of(from, to, now).Kind; k != Month {
				t.Errorf("Step(%d) produced Kind %v, want Month — the label would not match the rows", tc.delta, k)
			}
		})
	}
}

func TestStepRoundTrips(t *testing.T) {
	from, to := MonthBounds(now)
	for i := 0; i < 14; i++ {
		from, to = Of(from, to, now).Step(1)
	}
	for i := 0; i < 14; i++ {
		from, to = Of(from, to, now).Step(-1)
	}
	wantFrom, wantTo := MonthBounds(now)
	if from != wantFrom || to != wantTo {
		t.Errorf("14 months forward then back = %q..%q, want %q..%q", from, to, wantFrom, wantTo)
	}
}

func TestIsCurrentMonth(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		want     bool
	}{
		{"this month", "2026-08-01", "2026-08-31", true},
		{"last month", "2026-07-01", "2026-07-31", false},
		{"a day inside this month is not the month", "2026-08-16", "2026-08-16", false},
		{"all dates is not a month", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Of(tc.from, tc.to, now).IsCurrentMonth(now); got != tc.want {
				t.Errorf("IsCurrentMonth() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Bounds carrying a time of day (a stored value that was never day-truncated)
// must still classify as a whole month, or the label silently falls back to a
// range for a scope the user set from the month stepper.
func TestOfTruncatesBoundsToTheDay(t *testing.T) {
	if got := Of("2026-08-01", "2026-08-31", now); got.Kind != Month {
		t.Fatalf("Kind = %v, want Month", got.Kind)
	}
	s := Of("2026-08-01", "2026-08-31", now)
	if !s.From.Equal(day(2026, time.August, 1)) || !s.To.Equal(day(2026, time.August, 31)) {
		t.Errorf("bounds = %v..%v, want day-truncated August 1..31", s.From, s.To)
	}
}
