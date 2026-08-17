// SPDX-License-Identifier: MIT

package spendpattern

import (
	"testing"
	"time"
)

func mk(start time.Time, n int, minor int64) []Day {
	out := make([]Day, 0, n)
	for i := range n {
		out = append(out, Day{Date: start.AddDate(0, 0, i), Minor: minor})
	}
	return out
}

// days builds one day per date over span, spending weekendMinor on Sat/Sun and
// weekdayMinor otherwise.
func days(start time.Time, span int, weekendMinor, weekdayMinor int64) []Day {
	out := make([]Day, 0, span)
	for i := range span {
		d := start.AddDate(0, 0, i)
		m := weekdayMinor
		if IsWeekend(d) {
			m = weekendMinor
		}
		out = append(out, Day{Date: d, Minor: m})
	}
	return out
}

var jan1 = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// THE trap: five weekdays against two weekend days means TOTALS find "you spend
// more on weekdays" in every household that has ever existed.
func TestItComparesRatesNotTotals(t *testing.T) {
	// Identical daily spending everywhere. Totals would say weekdays win 5:2.
	ds := days(jan1, 120, 1_000, 1_000)
	in, out := SplitBy(ds, IsWeekend)
	f := Compare(in, out, MonthKey)
	if !f.Known {
		t.Fatal("expected a comparison")
	}
	if f.LiftPct != 0 {
		t.Errorf("lift = %.1f%%, want 0 — the same daily spend is not a pattern", f.LiftPct)
	}
	if f.Holds() {
		t.Error("flat spending was reported as a habit")
	}
	if f.OutsideDays <= f.InsideDays {
		t.Fatal("setup: expected more weekdays than weekend days")
	}
}

func TestARealWeekendHabitIsFound(t *testing.T) {
	ds := days(jan1, 120, 3_000, 1_000)
	in, out := SplitBy(ds, IsWeekend)
	f := Compare(in, out, MonthKey)
	if !f.Holds() {
		t.Fatalf("a 3x weekend rate was not reported: %+v", f)
	}
	if f.LiftPct < 190 || f.LiftPct > 210 {
		t.Errorf("lift = %.1f%%, want about 200", f.LiftPct)
	}
	if f.ExtraPerDayMinor() != 2_000 {
		t.Errorf("extra = %d, want 2000 — the figure somebody can act on", f.ExtraPerDayMinor())
	}
}

// One expensive weekend is an anomaly, and the app already detects those. A
// habit has to keep happening, or somebody changes a routine that was never the
// problem.
func TestOneLoudMonthIsNotAHabit(t *testing.T) {
	var ds []Day
	for m := range 4 {
		start := jan1.AddDate(0, m, 0)
		if m == 0 {
			ds = append(ds, days(start, 28, 20_000, 1_000)...) // the blowout month
			continue
		}
		ds = append(ds, days(start, 28, 950, 1_000)...) // quieter weekends after
	}
	in, out := SplitBy(ds, IsWeekend)
	f := Compare(in, out, MonthKey)
	if f.LiftPct < MinLiftPct {
		t.Fatalf("setup: the pooled average should still look big (%.1f%%)", f.LiftPct)
	}
	if f.Consistent {
		t.Errorf("one month carried the average and was called consistent (%d/%d periods)",
			f.PeriodsHeld, f.Periods)
	}
	if f.Holds() {
		t.Error("a one-off was reported as a habit")
	}
}

func TestASmallDifferenceIsNotWorthSaying(t *testing.T) {
	ds := days(jan1, 120, 1_100, 1_000) // 10% — real, and not worth a sentence
	in, out := SplitBy(ds, IsWeekend)
	f := Compare(in, out, MonthKey)
	if !f.Known {
		t.Fatal("expected a comparison")
	}
	if f.Holds() {
		t.Errorf("a %.0f%% difference was surfaced as a habit", f.LiftPct)
	}
}

// "No pattern" and "we could not look" are different answers, and only the first
// is a finding.
func TestTooLittleDataIsNotNoPattern(t *testing.T) {
	f := Compare(mk(jan1, 3, 5_000), mk(jan1, 40, 1_000), MonthKey)
	if f.Known {
		t.Errorf("three days was treated as evidence: %+v", f)
	}
	if f.LiftPct != 0 || f.ExtraPerDayMinor() != 0 {
		t.Error("an unknown finding carried numbers")
	}
	if Compare(mk(jan1, 40, 5_000), mk(jan1, 2, 1_000), MonthKey).Known {
		t.Error("two days on the other side was treated as evidence")
	}
}

func TestSpendingNothingOutsideThePhaseIsNotInfinitelyMore(t *testing.T) {
	f := Compare(mk(jan1, 30, 5_000), mk(jan1, 30, 0), MonthKey)
	if !f.Known {
		t.Fatal("expected a comparison")
	}
	if f.LiftPct != 0 {
		t.Errorf("lift = %v, want 0 — 'infinitely more' is not actionable", f.LiftPct)
	}
}

func TestPaydayPhaseCountsTheDayItself(t *testing.T) {
	pay := []time.Time{jan1, jan1.AddDate(0, 0, 14)}
	in := WithinDaysAfter(pay, 2)
	if !in(jan1) {
		t.Error("payday itself was outside the phase — money leaving the moment it lands is the question")
	}
	if !in(jan1.AddDate(0, 0, 2)) {
		t.Error("the second day after payday was outside a 2-day window")
	}
	if in(jan1.AddDate(0, 0, 3)) {
		t.Error("the third day was inside a 2-day window")
	}
	if !in(jan1.AddDate(0, 0, 15)) {
		t.Error("the second payday's window was missed")
	}
}

func TestPaydayHabitIsFound(t *testing.T) {
	// Paid on the 1st and 15th; the three days after each run hot.
	var pay []time.Time
	for m := range 4 {
		pay = append(pay, jan1.AddDate(0, m, 0), jan1.AddDate(0, m, 14))
	}
	phase := WithinDaysAfter(pay, 3)
	var ds []Day
	for i := range 120 {
		d := jan1.AddDate(0, 0, i)
		m := int64(1_000)
		if phase(d) {
			m = 4_000
		}
		ds = append(ds, Day{Date: d, Minor: m})
	}
	in, out := SplitBy(ds, phase)
	f := Compare(in, out, MonthKey)
	if !f.Holds() {
		t.Fatalf("a payday habit was not reported: %+v", f)
	}
	if f.PeriodsHeld != f.Periods {
		t.Errorf("held in %d of %d periods, want all", f.PeriodsHeld, f.Periods)
	}
}

// Without a period function there is nothing to be consistent across, and the
// pooled average alone is exactly the evidence this package refuses to treat as
// a habit.
func TestNoPeriodFunctionMeansNoHabitClaim(t *testing.T) {
	ds := days(jan1, 120, 3_000, 1_000)
	in, out := SplitBy(ds, IsWeekend)
	f := Compare(in, out, nil)
	if !f.Known || f.LiftPct < 100 {
		t.Fatalf("expected the lift to still be measured: %+v", f)
	}
	if f.Consistent || f.Holds() {
		t.Error("a pooled average alone was called a habit")
	}
}

func TestRefundsDoNotCancelSpending(t *testing.T) {
	// Days arrive as magnitudes; a negative entry is still a day money moved.
	f := Compare([]Day{
		{Date: jan1, Minor: -5_000}, {Date: jan1.AddDate(0, 0, 1), Minor: 5_000},
		{Date: jan1.AddDate(0, 0, 2), Minor: 5_000}, {Date: jan1.AddDate(0, 0, 3), Minor: 5_000},
		{Date: jan1.AddDate(0, 0, 4), Minor: 5_000}, {Date: jan1.AddDate(0, 0, 5), Minor: 5_000},
		{Date: jan1.AddDate(0, 0, 6), Minor: 5_000}, {Date: jan1.AddDate(0, 0, 7), Minor: 5_000},
	}, mk(jan1, 30, 1_000), nil)
	if f.InsideRateMinor != 5_000 {
		t.Errorf("inside rate = %d, want 5000 — a refund is not negative spending here", f.InsideRateMinor)
	}
}

func TestFindingsAreStable(t *testing.T) {
	ds := days(jan1, 120, 3_000, 1_000)
	in, out := SplitBy(ds, IsWeekend)
	first := Compare(in, out, MonthKey)
	for i := range 5 {
		if got := Compare(in, out, MonthKey); got != first {
			t.Fatalf("run %d differed: %+v vs %+v", i, got, first)
		}
	}
}
