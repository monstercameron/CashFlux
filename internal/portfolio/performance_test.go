// SPDX-License-Identifier: MIT

package portfolio

import (
	"math"
	"testing"
	"time"
)

func perfDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func perfNear(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// A portfolio that doubled tells you nothing about performance — it doubles the
// same way whether the market rose or you paid more in. This is the case the
// whole feature exists for.
func TestContributionsAreNotPerformance(t *testing.T) {
	// $10,000 in, another $10,000 contributed, ends at $20,000: the investments
	// did NOTHING, and a balance chart would show a doubling.
	vals := []Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.July, 1), ValueMinor: 1000000},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 2000000},
	}
	flows := []Flow{{Date: perfDay(2026, time.July, 2), AmountMinor: 1000000}}

	p := Returns(vals, flows)
	if !p.TWRKnown {
		t.Fatalf("TWR unknown: %+v", p)
	}
	if !perfNear(p.TimeWeightedPct, 0, 0.5) {
		t.Errorf("TimeWeightedPct = %v, want ~0 — the growth was entirely contributed",
			p.TimeWeightedPct)
	}
}

// TWR removes the timing of money; IRR includes it. They must be able to differ,
// or one of them is not being computed.
func TestTimeAndMoneyWeightedDiffer(t *testing.T) {
	// Flat first half, strong second half, with a large contribution landing just
	// before the strong half — good timing, so the investor beats the fund.
	vals := []Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.July, 1), ValueMinor: 1000000},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 4400000},
	}
	flows := []Flow{{Date: perfDay(2026, time.July, 2), AmountMinor: 3000000}}

	p := Returns(vals, flows)
	if !p.TWRKnown || !p.IRRKnown {
		t.Fatalf("both should be known: %+v", p)
	}
	if perfNear(p.TimeWeightedPct, p.MoneyWeightedPct, 0.01) {
		t.Errorf("TWR %v and IRR %v are identical — one is not being computed",
			p.TimeWeightedPct, p.MoneyWeightedPct)
	}
	gap, ok := p.GapPct()
	if !ok {
		t.Fatal("GapPct unknown when both figures are known")
	}
	if !perfNear(gap, p.MoneyWeightedPct-p.TimeWeightedPct, 1e-9) {
		t.Errorf("GapPct = %v", gap)
	}
}

// A gap computed against a missing number is not a gap.
func TestGapNeedsBothFigures(t *testing.T) {
	if _, ok := (Performance{TWRKnown: true}).GapPct(); ok {
		t.Error("GapPct reported with no IRR")
	}
	if _, ok := (Performance{IRRKnown: true}).GapPct(); ok {
		t.Error("GapPct reported with no TWR")
	}
}

// Annualizing scales BOTH ways: a six-month result up, a six-year result down.
// The point is comparability, not flattering a short window.
func TestReturnsAreAnnualized(t *testing.T) {
	// +10% over roughly half a year annualizes to about +21%.
	half := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.July, 2), ValueMinor: 1100000},
	}, nil)
	if !half.TWRKnown {
		t.Fatal("TWR unknown")
	}
	if half.TimeWeightedPct < 18 || half.TimeWeightedPct > 24 {
		t.Errorf("six-month +10%% annualized to %v, want ~21", half.TimeWeightedPct)
	}
	// +100% over ten years annualizes to about +7.2%, not +100%.
	long := Returns([]Valuation{
		{Date: perfDay(2016, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.January, 1), ValueMinor: 2000000},
	}, nil)
	if long.TimeWeightedPct < 6.5 || long.TimeWeightedPct > 7.5 {
		t.Errorf("ten-year doubling annualized to %v, want ~7.2", long.TimeWeightedPct)
	}
}

// Annualizing three good days multiplies their noise by the same factor it
// multiplies the return, and a spectacular figure is one people act on.
//
// The window still HAPPENED, though, so what it returned is reported over the
// window (FP-T1c-b) — what is refused is stating it as a yearly rate.
func TestVeryShortPeriodsAreNotAnnualized(t *testing.T) {
	p := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.January, 4), ValueMinor: 1100000},
	}, nil)
	if p.TWRKnown || p.IRRKnown {
		t.Errorf("a three-day window produced a yearly rate: %+v", p)
	}
	if p.Annualized {
		t.Error("a three-day window must not claim to be annualized")
	}
	if !p.PeriodKnown {
		t.Fatal("the window still happened — its return over the window must be reported")
	}
	if p.PeriodPct < 9.9 || p.PeriodPct > 10.1 {
		t.Errorf("period return = %v, want about 10%% over the window", p.PeriodPct)
	}
	// Annualizing that 10%% over three days would be roughly 300,000%. The gap
	// between the two numbers is the entire reason for the floor.
	if p.TimeWeightedPct != 0 {
		t.Errorf("TimeWeightedPct = %v, want it left at zero and unclaimed", p.TimeWeightedPct)
	}
}

// Two valuations on the same day describe an instant, and an instant has no
// return — that is not a short window, it is no window.
func TestNoElapsedTimeReportsNothingAtAll(t *testing.T) {
	p := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1100000},
	}, nil)
	if p.PeriodKnown || p.TWRKnown || p.IRRKnown || p.Days != 0 {
		t.Errorf("an instant produced a return: %+v", p)
	}
}

// A long window reports BOTH, and the annualized figure must be the smaller of
// the two for a multi-year run — that is what annualizing a cumulative return
// does, and getting it backwards would flatter every long holding.
func TestALongWindowReportsBothAndAnnualizingScalesDown(t *testing.T) {
	p := Returns([]Valuation{
		{Date: perfDay(2020, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.January, 1), ValueMinor: 2000000},
	}, nil)
	if !p.PeriodKnown || !p.TWRKnown || !p.Annualized {
		t.Fatalf("a six-year window must report both: %+v", p)
	}
	if p.PeriodPct < 99 || p.PeriodPct > 101 {
		t.Errorf("period return = %v, want about 100%% over six years", p.PeriodPct)
	}
	if p.TimeWeightedPct >= p.PeriodPct {
		t.Errorf("annualized %v should be well below the cumulative %v", p.TimeWeightedPct, p.PeriodPct)
	}
}

// A contribution made before the period began is not part of this period's
// return; folding it into an endpoint attributes a strong quarter to a deposit
// made a year earlier.
func TestFlowsOutsideTheWindowAreIgnored(t *testing.T) {
	vals := []Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 1100000},
	}
	withOutside := Returns(vals, []Flow{
		{Date: perfDay(2025, time.June, 1), AmountMinor: 5000000},
		{Date: perfDay(2028, time.June, 1), AmountMinor: 5000000},
	})
	clean := Returns(vals, nil)
	if withOutside.Flows != 0 {
		t.Errorf("Flows = %d, want 0 — both flows are outside the window", withOutside.Flows)
	}
	if !perfNear(withOutside.TimeWeightedPct, clean.TimeWeightedPct, 1e-9) {
		t.Errorf("out-of-window flows changed the return: %v vs %v",
			withOutside.TimeWeightedPct, clean.TimeWeightedPct)
	}
}

// A zero-amount flow is not a movement; counting it would inflate the provenance
// figure the surface shows ("3 contributions" when there were two).
func TestZeroFlowsAreNotCounted(t *testing.T) {
	p := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 1100000},
	}, []Flow{
		{Date: perfDay(2026, time.June, 1), AmountMinor: 0},
		{Date: perfDay(2026, time.July, 1), AmountMinor: 10000},
	})
	if p.Flows != 1 {
		t.Errorf("Flows = %d, want 1", p.Flows)
	}
}

// An account funded from nothing has no percentage return over the interval in
// which it was funded; the alternative is a division by zero propagating through
// the whole chain.
func TestASubPeriodStartingAtZeroIsSkipped(t *testing.T) {
	p := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 0},
		{Date: perfDay(2026, time.April, 1), ValueMinor: 1000000},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 1100000},
	}, []Flow{{Date: perfDay(2026, time.March, 1), AmountMinor: 1000000}})

	if !p.TWRKnown {
		t.Fatalf("TWR unknown: %+v", p)
	}
	if math.IsInf(p.TimeWeightedPct, 0) || math.IsNaN(p.TimeWeightedPct) {
		t.Errorf("TimeWeightedPct = %v", p.TimeWeightedPct)
	}
	// The measured stretch is April→January: +10% over ~9 months.
	if p.TimeWeightedPct < 10 || p.TimeWeightedPct > 16 {
		t.Errorf("TimeWeightedPct = %v, want the funded stretch's return", p.TimeWeightedPct)
	}
}

// Reporting -100% for a portfolio that later recovered is worse than reporting
// nothing.
func TestATotalLossInsideAPeriodIsNotReported(t *testing.T) {
	p := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.June, 1), ValueMinor: 0},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 1000000},
	}, []Flow{{Date: perfDay(2026, time.July, 1), AmountMinor: 1000000}})
	if p.TWRKnown {
		t.Errorf("a wipe-and-refund reported a time-weighted return of %v", p.TimeWeightedPct)
	}
}

func TestGuards(t *testing.T) {
	if p := Returns(nil, nil); p.TWRKnown || p.IRRKnown {
		t.Error("no valuations produced a return")
	}
	if p := Returns([]Valuation{{Date: perfDay(2026, time.January, 1), ValueMinor: 100}}, nil); p.TWRKnown {
		t.Error("a single valuation produced a return")
	}
}

// Provenance: a return with no evidence behind it is a number to be believed
// rather than checked.
func TestPerformanceReportsItsEvidence(t *testing.T) {
	p := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.July, 1), ValueMinor: 1050000},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 1100000},
	}, []Flow{{Date: perfDay(2026, time.March, 1), AmountMinor: 10000}})

	if p.Valuations != 3 || p.Flows != 1 {
		t.Errorf("evidence = %d valuations / %d flows", p.Valuations, p.Flows)
	}
	if p.Days < 360 || p.Days > 372 {
		t.Errorf("Days = %d, want about a year", p.Days)
	}
}

// Unsorted input must not change the answer — a caller assembling valuations
// from several sources should not have to sort them first.
func TestInputOrderDoesNotMatter(t *testing.T) {
	ordered := Returns([]Valuation{
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.July, 1), ValueMinor: 1050000},
		{Date: perfDay(2027, time.January, 1), ValueMinor: 1100000},
	}, nil)
	shuffled := Returns([]Valuation{
		{Date: perfDay(2027, time.January, 1), ValueMinor: 1100000},
		{Date: perfDay(2026, time.January, 1), ValueMinor: 1000000},
		{Date: perfDay(2026, time.July, 1), ValueMinor: 1050000},
	}, nil)
	if !perfNear(ordered.TimeWeightedPct, shuffled.TimeWeightedPct, 1e-9) {
		t.Errorf("order changed the answer: %v vs %v",
			ordered.TimeWeightedPct, shuffled.TimeWeightedPct)
	}
}
