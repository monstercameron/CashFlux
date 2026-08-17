// SPDX-License-Identifier: MIT

package esttax

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestQuartersAreNotCalendarQuarters(t *testing.T) {
	// The detail that catches people: the periods run Jan–Mar, Apr–May, Jun–Aug,
	// Sep–Dec. Treating them as calendar quarters puts two deadlines in the wrong
	// place.
	cases := []struct {
		when time.Time
		q    int
		due  time.Time
	}{
		{d(2026, time.January, 5), 1, d(2026, time.April, 15)},
		{d(2026, time.March, 31), 1, d(2026, time.April, 15)},
		{d(2026, time.April, 1), 2, d(2026, time.June, 15)},
		{d(2026, time.May, 31), 2, d(2026, time.June, 15)},
		// June is the third period, not the second — a calendar reading gets this
		// one wrong.
		{d(2026, time.June, 1), 3, d(2026, time.September, 15)},
		{d(2026, time.August, 31), 3, d(2026, time.September, 15)},
		{d(2026, time.September, 1), 4, d(2027, time.January, 15)},
		// The fourth payment is due in JANUARY of the following year.
		{d(2026, time.December, 31), 4, d(2027, time.January, 15)},
	}
	for _, c := range cases {
		q, due := QuarterOf(c.when)
		if q != c.q || !due.Equal(c.due) {
			t.Errorf("%s: got Q%d due %s, want Q%d due %s",
				c.when.Format("2 Jan"), q, due.Format("2 Jan 2006"), c.q, c.due.Format("2 Jan 2006"))
		}
	}
}

func TestProjectionScalesIncomeByElapsedDays(t *testing.T) {
	// Half a year of $50,000 at 25% projects to $100,000 of income and $25,000 of
	// tax.
	in := Inputs{
		NetIncomeMinor: 5_000_000, EffectiveRatePct: 25,
		Now: d(2026, time.July, 2), // day 183 of 365
	}
	e, ok := Compute(in)
	if !ok {
		t.Fatal("expected an estimate")
	}
	if e.ProjectedTaxMinor < 2_450_000 || e.ProjectedTaxMinor > 2_550_000 {
		t.Errorf("projected tax = %d, want about 2500000", e.ProjectedTaxMinor)
	}
}

func TestProjectionDoesNotJumpAtAQuarterBoundary(t *testing.T) {
	// Scaling by quarter rather than by day would step the projection by a third
	// overnight on no new information.
	base := Inputs{NetIncomeMinor: 5_000_000, EffectiveRatePct: 25}
	before := base
	before.Now = d(2026, time.May, 31)
	after := base
	after.Now = d(2026, time.June, 1)
	e1, ok1 := Compute(before)
	e2, ok2 := Compute(after)
	if !ok1 || !ok2 {
		t.Fatal("expected both estimates")
	}
	diff := e1.ProjectedTaxMinor - e2.ProjectedTaxMinor
	if diff < 0 {
		diff = -diff
	}
	// One day's worth of change, not a quarter's.
	if diff > e1.ProjectedTaxMinor/50 {
		t.Errorf("a one-day step moved the projection by %d (from %d) — that is a jump, not a day",
			diff, e1.ProjectedTaxMinor)
	}
}

func TestSafeHarborUsesLastYearsTaxAndItsTier(t *testing.T) {
	in := Inputs{
		NetIncomeMinor: 5_000_000, EffectiveRatePct: 25, Now: d(2026, time.July, 2),
		PriorYearTaxMinor: 2_000_000, PriorYearIncomeMinor: 10_000_000,
	}
	e, _ := Compute(in)
	if !e.SafeHarborKnown || e.SafeHarborPct != SafeHarborPct || e.SafeHarborMinor != 2_000_000 {
		t.Errorf("safe harbor = %d at %v%%, want 2000000 at %v%%",
			e.SafeHarborMinor, e.SafeHarborPct, SafeHarborPct)
	}
	// Above the threshold the higher share applies.
	in.PriorYearIncomeMinor = HighIncomeThresholdMinor + 1
	e2, _ := Compute(in)
	if e2.SafeHarborPct != SafeHarborHighPct || e2.SafeHarborMinor != 2_200_000 {
		t.Errorf("high-tier safe harbor = %d at %v%%, want 2200000 at %v%%",
			e2.SafeHarborMinor, e2.SafeHarborPct, SafeHarborHighPct)
	}
}

func TestAnUnknownPriorYearRemovesTheHarborRatherThanAssumingZero(t *testing.T) {
	// Treating an unknown prior year as zero tax would produce a $0 safe harbor —
	// which, being the lower of the two, would then become the target and tell
	// someone to pay nothing.
	in := Inputs{NetIncomeMinor: 5_000_000, EffectiveRatePct: 25, Now: d(2026, time.July, 2)}
	e, _ := Compute(in)
	if e.SafeHarborKnown {
		t.Error("no prior-year tax must mean no safe harbor")
	}
	if e.TargetMinor != e.ProjectedTaxMinor {
		t.Errorf("target = %d, want the projection %d when there is no harbor",
			e.TargetMinor, e.ProjectedTaxMinor)
	}
}

func TestTargetTakesTheLowerOfProjectionAndHarbor(t *testing.T) {
	// Either satisfies the rule, so asking for the larger is asking for an
	// interest-free loan.
	in := Inputs{
		NetIncomeMinor: 5_000_000, EffectiveRatePct: 25, Now: d(2026, time.July, 2),
		PriorYearTaxMinor: 500_000,
	}
	e, _ := Compute(in)
	if e.TargetMinor != e.SafeHarborMinor {
		t.Errorf("target = %d, want the cheaper harbor %d", e.TargetMinor, e.SafeHarborMinor)
	}
}

func TestDueNowIsWhatTheQuarterOwesLessWhatWasPaid(t *testing.T) {
	in := Inputs{
		NetIncomeMinor: 5_000_000, EffectiveRatePct: 25, Now: d(2026, time.July, 2),
		PriorYearTaxMinor: 2_000_000, PaidToDateMinor: 500_000,
	}
	e, _ := Compute(in)
	// Q3 of a $20,000 target = $15,000 should have gone; $5,000 has.
	if e.Quarter != 3 {
		t.Fatalf("quarter = %d, want 3", e.Quarter)
	}
	if e.DueNowMinor != 1_000_000 {
		t.Errorf("due now = %d, want 1000000", e.DueNowMinor)
	}
}

func TestBeingAheadIsReportedAsAheadNotAsZero(t *testing.T) {
	in := Inputs{
		NetIncomeMinor: 5_000_000, EffectiveRatePct: 25, Now: d(2026, time.February, 1),
		PriorYearTaxMinor: 2_000_000, PaidToDateMinor: 1_500_000,
	}
	e, _ := Compute(in)
	if e.DueNowMinor >= 0 {
		t.Errorf("due now = %d, want a negative figure for someone paid ahead", e.DueNowMinor)
	}
}

func TestRefusalsRatherThanConfidentZeroes(t *testing.T) {
	cases := []struct {
		name string
		in   Inputs
	}{
		{"no rate", Inputs{NetIncomeMinor: 1_000_000, Now: d(2026, time.July, 2)}},
		{"absurd rate", Inputs{NetIncomeMinor: 1_000_000, EffectiveRatePct: 150, Now: d(2026, time.July, 2)}},
		// A year with no income yet is not a year that owes nothing — a confident
		// "$0 due" reads as a green light.
		{"no income yet", Inputs{EffectiveRatePct: 25, Now: d(2026, time.July, 2)}},
		{"a loss so far", Inputs{NetIncomeMinor: -100, EffectiveRatePct: 25, Now: d(2026, time.July, 2)}},
		{"no date", Inputs{NetIncomeMinor: 1_000_000, EffectiveRatePct: 25}},
	}
	for _, c := range cases {
		if _, ok := Compute(c.in); ok {
			t.Errorf("%s: expected a refusal", c.name)
		}
	}
}

func TestLeapYearsScaleOverThreeSixtySix(t *testing.T) {
	// 2028 is a leap year; the same day-of-year covers a slightly smaller share
	// of it, so the projection is slightly larger.
	in := Inputs{NetIncomeMinor: 5_000_000, EffectiveRatePct: 25, Now: d(2028, time.July, 2)}
	e, ok := Compute(in)
	if !ok {
		t.Fatal("expected an estimate")
	}
	plain := Inputs{NetIncomeMinor: 5_000_000, EffectiveRatePct: 25, Now: d(2026, time.July, 2)}
	p, _ := Compute(plain)
	if e.ProjectedTaxMinor == p.ProjectedTaxMinor {
		t.Error("a leap year must scale over 366 days, not 365")
	}
}
