// SPDX-License-Identifier: MIT

package watchlist

import "testing"

func TestSignalsAcrossTheBand(t *testing.T) {
	cases := []struct {
		name   string
		total  int64
		target int64
		want   Signal
	}{
		{"well under", 10_000, 100_000, SignalUnder},
		{"just below the near band", 79_000, 100_000, SignalUnder},
		{"at the near band", 80_000, 100_000, SignalNear},
		{"nearly there", 95_000, 100_000, SignalNear},
		{"exactly at the target is not yet over", 100_000, 100_000, SignalNear},
		{"a dollar over", 100_100, 100_000, SignalOver},
	}
	for _, c := range cases {
		if got := Evaluate(c.total, c.target).Signal; got != c.want {
			t.Errorf("%s: signal = %q, want %q", c.name, got, c.want)
		}
	}
}

// A watchlist with no target is not doing well, it is not being measured — and
// reporting it as "under" answers a question nobody asked.
func TestNoTargetIsNotTheSameAsDoingWell(t *testing.T) {
	for _, target := range []int64{0, -1} {
		s := Evaluate(50_000, target)
		if s.Signal != SignalNone {
			t.Errorf("target %d gave signal %q, want %q", target, s.Signal, SignalNone)
		}
		if s.PctOfTarget != 0 || s.RemainingMinor != 0 {
			t.Errorf("target %d produced figures against a target that does not exist: %+v", target, s)
		}
	}
}

func TestRemainingGoesNegativePastTheTarget(t *testing.T) {
	// "You are $40 over" is the fact. A clamped zero would read as "you have
	// nothing left", which is softer and different.
	s := Evaluate(104_000, 100_000)
	if s.RemainingMinor != -4_000 {
		t.Errorf("remaining = %d, want -4000", s.RemainingMinor)
	}
	s2 := Evaluate(60_000, 100_000)
	if s2.RemainingMinor != 40_000 {
		t.Errorf("remaining = %d, want 40000", s2.RemainingMinor)
	}
}

func TestPercentOfTarget(t *testing.T) {
	if got := Evaluate(25_000, 100_000).PctOfTarget; got != 25 {
		t.Errorf("pct = %v, want 25", got)
	}
	if got := Evaluate(150_000, 100_000).PctOfTarget; got != 150 {
		t.Errorf("pct = %v, want 150", got)
	}
}

func TestAverageNeedsEnoughPeriodsToMeanAnything(t *testing.T) {
	// Two periods give an average either of them can swing entirely, and calling
	// that "normal" invents a baseline out of a coincidence.
	if _, ok := Compare(50_000, []int64{40_000, 60_000}); ok {
		t.Errorf("two prior periods produced an average; %d are required", MinPeriodsForAverage)
	}
	if _, ok := Compare(50_000, nil); ok {
		t.Error("no prior periods must not produce an average")
	}
	if _, ok := Compare(50_000, []int64{40_000, 60_000, 50_000}); !ok {
		t.Error("three prior periods must be enough")
	}
}

func TestComparisonReportsTheDirectionAndTheSpread(t *testing.T) {
	c, ok := Compare(60_000, []int64{40_000, 50_000, 45_000, 45_000})
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.AverageMinor != 45_000 {
		t.Errorf("average = %d, want 45000", c.AverageMinor)
	}
	if c.DeltaMinor != 15_000 {
		t.Errorf("delta = %d, want a positive 15000 for spending more than usual", c.DeltaMinor)
	}
	if c.Periods != 4 {
		t.Errorf("periods = %d, want 4 so a surface can name the window", c.Periods)
	}
	if c.DeltaPct < 33 || c.DeltaPct > 34 {
		t.Errorf("delta pct = %v, want about 33", c.DeltaPct)
	}
}

func TestSpendingLessThanUsualIsNegative(t *testing.T) {
	c, ok := Compare(30_000, []int64{50_000, 50_000, 50_000})
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.DeltaMinor >= 0 || c.DeltaPct >= 0 {
		t.Errorf("spending less must read negative: %+v", c)
	}
	if !c.Unusual() {
		t.Error("40% below usual is worth a sentence")
	}
}

func TestOrdinaryFluctuationIsNotCalledUnusual(t *testing.T) {
	// Spending is lumpy. A monitor that flags every ordinary month teaches people
	// to ignore it.
	c, ok := Compare(52_000, []int64{50_000, 50_000, 50_000})
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.Unusual() {
		t.Errorf("4%% above usual (%.1f%%) must not be flagged", c.DeltaPct)
	}
}

func TestAZeroBaselineRefusesRatherThanDividing(t *testing.T) {
	// A percentage against a zero baseline is not a large percentage, it is a
	// division nobody should have done.
	if _, ok := Compare(50_000, []int64{0, 0, 0}); ok {
		t.Error("a zero average must refuse")
	}
}

func TestAveragingIsResilientToOneOutlier(t *testing.T) {
	// One enormous month should move the average without defining it — the check
	// that the baseline is a mean over enough periods rather than a latest-value.
	c, ok := Compare(50_000, []int64{50_000, 50_000, 50_000, 250_000})
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.AverageMinor <= 50_000 || c.AverageMinor >= 250_000 {
		t.Errorf("average = %d, want it pulled up but not dominated", c.AverageMinor)
	}
}

// Spending totals are NEGATIVE in this ledger. A signed comparison read a $3,394
// spend against a $1 alert as 339,395% under — found by a browser check printing
// the number out loud, which is the kind of nonsense a unit test with tidy
// positive fixtures never produces.
func TestSpendingIsComparedByMagnitude(t *testing.T) {
	spent := Evaluate(-339_495, 100)
	if spent.Signal != SignalOver {
		t.Errorf("a $3,394 spend against a $1 alert = %q, want over", spent.Signal)
	}
	if spent.PctOfTarget < 0 {
		t.Errorf("pct = %v, want a positive share of the alert", spent.PctOfTarget)
	}
	if spent.RemainingMinor >= 0 {
		t.Errorf("remaining = %d, want negative — it is past the alert", spent.RemainingMinor)
	}
	// And it agrees with an equivalent positive total, or two screens disagree.
	if pos := Evaluate(339_495, 100); pos.Signal != spent.Signal || pos.PctOfTarget != spent.PctOfTarget {
		t.Errorf("signed and unsigned totals disagreed: %+v vs %+v", spent, pos)
	}
}

func TestMagnitudeComparisonAgreesWithTheExistingThresholdRule(t *testing.T) {
	// savedtxnview.CrossedThreshold has always taken the absolute value. If these
	// two drift, one screen says "over" while the other says "plenty left".
	cases := []struct{ total, target int64 }{
		{-50_000, 100_000}, {-100_000, 100_000}, {-150_000, 100_000},
	}
	for _, c := range cases {
		crossed := abs64(c.total) >= c.target
		over := Evaluate(c.total, c.target).Signal == SignalOver
		nearOrOver := over || Evaluate(c.total, c.target).Signal == SignalNear
		if crossed && !nearOrOver {
			t.Errorf("total %d vs target %d: threshold says crossed, watchlist does not", c.total, c.target)
		}
	}
}
