// SPDX-License-Identifier: MIT

package periodage

import (
	"testing"
	"time"
)

func at(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
}

// julyStart/julyEnd bracket a 31-day month, the case the ticket describes.
func july() (time.Time, time.Time) {
	return at(2026, time.July, 1, 0), at(2026, time.August, 1, 0)
}

func TestDayAndLengthReadNaturally(t *testing.T) {
	start, end := july()
	for _, tc := range []struct {
		now      time.Time
		wantDay  int
		wantDays int
	}{
		{at(2026, time.July, 1, 0), 1, 31},
		{at(2026, time.July, 1, 23), 1, 31},
		{at(2026, time.July, 3, 9), 3, 31},
		{at(2026, time.July, 31, 23), 31, 31},
	} {
		a := Of(start, end, tc.now)
		if a.Day != tc.wantDay || a.Days != tc.wantDays {
			t.Errorf("Of(%s) = day %d of %d, want day %d of %d",
				tc.now.Format(time.DateOnly), a.Day, a.Days, tc.wantDay, tc.wantDays)
		}
	}
}

// The ticket's exact scenario: day 3 of a month is not enough period to score.
func TestDayThreeOfAMonthIsEarly(t *testing.T) {
	start, end := july()
	if a := Of(start, end, at(2026, time.July, 3, 9)); !a.Early() {
		t.Errorf("day 3 of 31 (elapsed %.3f) is not Early — this is the reading that "+
			"produced \"every budget on track\" and \"budget adherence 100%%\"", a.Elapsed)
	}
	// By day 5 there is enough month to read, which is where the ticket drew it.
	if a := Of(start, end, at(2026, time.July, 5, 12)); a.Early() {
		t.Errorf("day 5 of 31 (elapsed %.3f) is still Early — the framing should have "+
			"stepped aside by now", a.Elapsed)
	}
}

// A weekly budget must not be held to a monthly budget's calendar: expressing
// the rule as a fraction is the reason this works.
func TestShortPeriodsLeaveEarlySooner(t *testing.T) {
	start := at(2026, time.July, 6, 0)
	end := start.AddDate(0, 0, 7)
	if a := Of(start, end, start.Add(12*time.Hour)); !a.Early() {
		t.Error("half a day into a week should still be early")
	}
	if a := Of(start, end, start.AddDate(0, 0, 1)); a.Early() {
		t.Errorf("a full day into a week (elapsed %.3f) should no longer be early", a.Elapsed)
	}
}

func TestCompletePeriodIsNeverEarly(t *testing.T) {
	start, end := july()
	a := Of(start, end, at(2026, time.August, 4, 0))
	if !a.Complete {
		t.Fatal("a period that has fully run reports Complete=false")
	}
	if a.Early() {
		t.Error("a finished period is Early — a final figure needs no framing")
	}
	if a.Elapsed != 1 {
		t.Errorf("Elapsed = %v, want 1", a.Elapsed)
	}
}

func TestDegeneratePeriodReadsComplete(t *testing.T) {
	now := at(2026, time.July, 3, 0)
	a := Of(now, now, now)
	if !a.Complete || a.Early() {
		t.Errorf("a zero-length period should read complete and not early, got %+v", a)
	}
	// An inverted period likewise — there is no partial window to warn about.
	if b := Of(now, now.AddDate(0, 0, -5), now); !b.Complete {
		t.Error("an inverted period should read complete")
	}
}

func TestProrateComparesLikeWithLike(t *testing.T) {
	start, end := july()
	// Three days into a 31-day month: ~9.7% elapsed.
	a := Of(start, end, at(2026, time.July, 4, 0))
	const lastMonthSpend = 310_000 // $3,100
	got := a.Prorate(lastMonthSpend)
	if got < 28_000 || got > 32_000 {
		t.Errorf("Prorate(%d) at %.3f elapsed = %d, want ~30000 — comparing three days "+
			"against a whole month is what produced \"spending is down 66%%\"",
			lastMonthSpend, a.Elapsed, got)
	}
	// A finished period compares against the whole reference.
	done := Of(start, end, at(2026, time.August, 1, 0))
	if got := done.Prorate(lastMonthSpend); got != lastMonthSpend {
		t.Errorf("Prorate on a complete period = %d, want the full %d", got, lastMonthSpend)
	}
	// Nothing elapsed → nothing to compare against.
	fresh := Of(start, end, start)
	if got := fresh.Prorate(lastMonthSpend); got != 0 {
		t.Errorf("Prorate at the instant a period opens = %d, want 0", got)
	}
}

func TestProrateWindowSlicesTheReference(t *testing.T) {
	start, end := july()
	a := Of(start, end, at(2026, time.July, 4, 0))
	refStart, refEnd := at(2026, time.June, 1, 0), at(2026, time.July, 1, 0)

	ws, we := a.ProrateWindow(refStart, refEnd)
	if !ws.Equal(refStart) {
		t.Errorf("window start = %s, want the reference start %s", ws, refStart)
	}
	if !we.After(refStart) || !we.Before(refEnd) {
		t.Fatalf("window end %s is not inside the reference period", we)
	}
	// ~9.7% of 30 days ≈ 2.9 days.
	if d := we.Sub(refStart).Hours() / 24; d < 2.5 || d > 3.5 {
		t.Errorf("window covers %.2f days of the reference, want ~2.9", d)
	}

	done := Of(start, end, at(2026, time.August, 1, 0))
	if ws, we := done.ProrateWindow(refStart, refEnd); !ws.Equal(refStart) || !we.Equal(refEnd) {
		t.Error("a complete period should compare against the whole reference window")
	}
	// A degenerate reference cannot be sliced.
	if ws, we := a.ProrateWindow(refStart, refStart); !ws.Equal(refStart) || !we.Equal(refStart) {
		t.Error("a zero-length reference window should come back unchanged")
	}
}
