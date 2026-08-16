// SPDX-License-Identifier: MIT

package txnscope

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// The properties below are what make the C560 guarantee hold for ranges nobody
// enumerated by hand. They are exhaustive over a multi-year window rather than
// random, so a failure names an exact date instead of a seed.

// Every month between 2020 and 2031 must round-trip: the bounds a month produces
// classify back as that same month. If this ever fails, the period bar shows a
// range where it should show a month name — which is the disagreement C560 fixed.
func TestEveryMonthRoundTripsAsAMonth(t *testing.T) {
	now := day(2026, time.August, 16)
	for y := 2020; y <= 2031; y++ {
		for m := time.January; m <= time.December; m++ {
			anchor := day(y, m, 1)
			from, to := MonthBounds(anchor)
			s := Of(from, to, now)
			if s.Kind != Month {
				t.Fatalf("%s: Of(%q, %q).Kind = %v, want Month", anchor.Format("2006-01"), from, to, s.Kind)
			}
			if !s.Anchor.Equal(anchor) {
				t.Fatalf("%s: Anchor = %v, want %v", anchor.Format("2006-01"), s.Anchor, anchor)
			}
			if got := s.To.Day(); got != dateutil.AddMonths(anchor, 1).AddDate(0, 0, -1).Day() {
				t.Fatalf("%s: To lands on day %d, not the month's last", anchor.Format("2006-01"), got)
			}
		}
	}
}

// Stepping forward then back must return to where it started, from any month.
// A stepper that drifts would silently walk the user off the month they meant.
func TestSteppingIsReversibleFromEveryMonth(t *testing.T) {
	now := day(2026, time.August, 16)
	for y := 2020; y <= 2031; y++ {
		for m := time.January; m <= time.December; m++ {
			from, to := MonthBounds(day(y, m, 1))
			f2, t2 := Of(from, to, now).Step(1)
			f3, t3 := Of(f2, t2, now).Step(-1)
			if f3 != from || t3 != to {
				t.Fatalf("%d-%02d: +1 then -1 = %q..%q, want %q..%q", y, m, f3, t3, from, to)
			}
		}
	}
}

// Stepping a whole year forward one month at a time must land exactly a year on,
// including across the two February lengths.
func TestTwelveStepsAdvanceOneYear(t *testing.T) {
	now := day(2026, time.August, 16)
	for _, start := range []time.Time{
		day(2023, time.January, 1),  // into a leap year
		day(2024, time.February, 1), // out of one
		day(2026, time.December, 1), // across a year boundary
	} {
		from, to := MonthBounds(start)
		for i := 0; i < 12; i++ {
			from, to = Of(from, to, now).Step(1)
		}
		wantFrom, wantTo := MonthBounds(dateutil.AddMonths(start, 12))
		if from != wantFrom || to != wantTo {
			t.Errorf("from %s: twelve steps = %q..%q, want %q..%q",
				start.Format("2006-01"), from, to, wantFrom, wantTo)
		}
	}
}

// Every day of a month must anchor on that month, so the calendar opens on the
// right grid after a day-click anywhere in it — including the 1st and the last.
func TestEveryDayAnchorsOnItsOwnMonth(t *testing.T) {
	now := day(2026, time.August, 16)
	for _, month := range []time.Time{
		day(2024, time.February, 1), // 29 days
		day(2026, time.February, 1), // 28 days
		day(2026, time.April, 1),    // 30 days
		day(2026, time.July, 1),     // 31 days
	} {
		last := dateutil.AddMonths(month, 1).AddDate(0, 0, -1)
		for d := month; !d.After(last); d = d.AddDate(0, 0, 1) {
			key := dateutil.FormatDate(d)
			s := Of(key, key, now)
			if s.Kind != Day {
				t.Fatalf("%s: Kind = %v, want Day", key, s.Kind)
			}
			if !s.Anchor.Equal(month) {
				t.Fatalf("%s: Anchor = %v, want %v", key, s.Anchor, month)
			}
		}
	}
}

// A single-day scope must never be mistaken for a month, even on a one-day
// month boundary — the 1st of a month is a Day, not that month.
func TestFirstOfMonthAloneIsADayNotAMonth(t *testing.T) {
	now := day(2026, time.August, 16)
	if got := Of("2026-08-01", "2026-08-01", now).Kind; got != Day {
		t.Errorf("Kind = %v, want Day — a single day is not the month containing it", got)
	}
}

// A reversed range (To before From) is user-producible from the filter panel's
// two date inputs. It must classify as Custom and still anchor somewhere sane,
// never panic and never claim to be a month.
func TestReversedRangeIsCustomAndSafe(t *testing.T) {
	now := day(2026, time.August, 16)
	s := Of("2026-08-31", "2026-08-01", now)
	if s.Kind != Custom {
		t.Errorf("Kind = %v, want Custom", s.Kind)
	}
	if s.Anchor.IsZero() {
		t.Error("Anchor is zero — the calendar would have no month to draw")
	}
	// And it still steps onto a real month rather than compounding the mistake.
	from, to := s.Step(1)
	if k := Of(from, to, now).Kind; k != Month {
		t.Errorf("Step from a reversed range produced %v, want Month", k)
	}
}

// IsCurrentMonth must track the clock it is given, not a captured one.
func TestIsCurrentMonthFollowsTheSuppliedClock(t *testing.T) {
	from, to := MonthBounds(day(2026, time.August, 16))
	s := Of(from, to, day(2026, time.August, 16))
	if !s.IsCurrentMonth(day(2026, time.August, 1)) {
		t.Error("August scope should be current on Aug 1")
	}
	if !s.IsCurrentMonth(day(2026, time.August, 31)) {
		t.Error("August scope should be current on Aug 31")
	}
	if s.IsCurrentMonth(day(2026, time.September, 1)) {
		t.Error("August scope should not be current on Sep 1")
	}
}
