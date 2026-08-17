// SPDX-License-Identifier: MIT

package montecarlo

import (
	"math"
	"testing"
)

// The property the whole package rests on. A simulation that reports a different
// answer each time it is opened is a black box, and this app does not ship those.
func TestTheSameInputsAlwaysGiveTheSameAnswer(t *testing.T) {
	cfg := Config{MeanReturnPct: 7, StdDevPct: 15, InflationPct: 3}
	first, ok := Run(100_000_000, 4_000_000, 30, cfg)
	if !ok {
		t.Fatal("expected a result")
	}
	for i := range 5 {
		got, ok := Run(100_000_000, 4_000_000, 30, cfg)
		if !ok {
			t.Fatalf("run %d refused", i)
		}
		if got != first {
			t.Fatalf("run %d differed:\n got %+v\nwant %+v", i, got, first)
		}
	}
}

func TestADifferentSeedGivesADifferentAnswer(t *testing.T) {
	// If it did not, the seed would not be doing anything and "reproducible"
	// would be an accident rather than a property.
	a, _ := Run(100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7, Seed: 1})
	b, _ := Run(100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7, Seed: 2})
	if a == b {
		t.Error("two seeds produced identical results — the seed is not reaching the generator")
	}
}

func TestTheSeedAndIterationsAreReportedBack(t *testing.T) {
	// A figure nobody can reproduce is a figure nobody can dispute.
	r, ok := Run(100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7, Seed: 42, Iterations: 500})
	if !ok {
		t.Fatal("expected a result")
	}
	if r.Seed != 42 || r.Iterations != 500 {
		t.Errorf("got seed %d over %d iterations, want 42 over 500", r.Seed, r.Iterations)
	}
}

func TestDefaultsAreFilledInAndEchoed(t *testing.T) {
	r, ok := Run(100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7})
	if !ok {
		t.Fatal("expected a result")
	}
	if r.Iterations != DefaultIterations || r.Seed != DefaultSeed {
		t.Errorf("got %d iterations / seed %d, want the stated defaults", r.Iterations, r.Seed)
	}
}

func TestSpendingNothingAlwaysSucceeds(t *testing.T) {
	r, ok := Run(100_000_000, 0, 30, Config{MeanReturnPct: 7})
	if !ok {
		t.Fatal("expected a result")
	}
	if r.SuccessRatePct != 100 {
		t.Errorf("success = %v%%, want 100 when nothing is withdrawn", r.SuccessRatePct)
	}
	if r.Depleted {
		t.Error("nothing was withdrawn, so nothing can have run out")
	}
}

func TestSpendingEverythingImmediatelyAlwaysFails(t *testing.T) {
	// Spending comes out at the START of the year, so a withdrawal equal to the
	// balance leaves nothing to grow.
	r, ok := Run(10_000_000, 10_000_000, 30, Config{MeanReturnPct: 7})
	if !ok {
		t.Fatal("expected a result")
	}
	if r.SuccessRatePct != 0 {
		t.Errorf("success = %v%%, want 0", r.SuccessRatePct)
	}
	if r.WorstDepletionYear != 1 {
		t.Errorf("worst depletion year = %d, want 1", r.WorstDepletionYear)
	}
}

func TestSpendingMoreLowersTheSuccessRate(t *testing.T) {
	cfg := Config{MeanReturnPct: 7, StdDevPct: 15}
	light, _ := Run(100_000_000, 3_000_000, 30, cfg)
	heavy, _ := Run(100_000_000, 8_000_000, 30, cfg)
	if heavy.SuccessRatePct >= light.SuccessRatePct {
		t.Errorf("spending 8%% (%v%%) should succeed less often than 3%% (%v%%)",
			heavy.SuccessRatePct, light.SuccessRatePct)
	}
}

func TestVolatilityHurtsADrawdownAtTheSameAverageReturn(t *testing.T) {
	// Sequence-of-returns risk, and the reason a single-rate projection cannot
	// answer this question: the same average with a rougher ride fails more often.
	calm, _ := Run(100_000_000, 5_000_000, 30, Config{MeanReturnPct: 7, StdDevPct: 2})
	rough, _ := Run(100_000_000, 5_000_000, 30, Config{MeanReturnPct: 7, StdDevPct: 25})
	if rough.SuccessRatePct >= calm.SuccessRatePct {
		t.Errorf("volatile (%v%%) should fail more than calm (%v%%) at the same mean",
			rough.SuccessRatePct, calm.SuccessRatePct)
	}
}

func TestInflationShortensHowLongTheMoneyLasts(t *testing.T) {
	// A drawdown that ignores inflation overstates the horizon by more, the
	// longer it runs.
	none, _ := Run(100_000_000, 5_000_000, 30, Config{MeanReturnPct: 7, InflationPct: 0})
	some, _ := Run(100_000_000, 5_000_000, 30, Config{MeanReturnPct: 7, InflationPct: 4})
	if some.SuccessRatePct >= none.SuccessRatePct {
		t.Errorf("4%% inflation (%v%%) must not succeed as often as none (%v%%)",
			some.SuccessRatePct, none.SuccessRatePct)
	}
}

func TestZeroVolatilityMatchesADeterministicRun(t *testing.T) {
	// With no volatility every future is the same one, so the simulation must
	// agree with plain arithmetic — the sanity check that the machinery is not
	// doing something else entirely.
	r, ok := Run(100_000_000, 0, 10, Config{MeanReturnPct: 10, NoVolatility: true, Iterations: 50})
	if !ok {
		t.Fatal("expected a result")
	}
	want := 100_000_000 * math.Pow(1.10, 10)
	got := float64(r.MedianEndingMinor)
	if math.Abs(got-want)/want > 0.001 {
		t.Errorf("median ending = %v, want about %v", got, want)
	}
	if r.P10EndingMinor != r.P90EndingMinor {
		t.Errorf("with no volatility every run must end the same: p10 %d, p90 %d",
			r.P10EndingMinor, r.P90EndingMinor)
	}
}

func TestPercentilesAreOrdered(t *testing.T) {
	r, ok := Run(100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7, StdDevPct: 15})
	if !ok {
		t.Fatal("expected a result")
	}
	if !(r.P10EndingMinor <= r.MedianEndingMinor && r.MedianEndingMinor <= r.P90EndingMinor) {
		t.Errorf("percentiles out of order: p10 %d, median %d, p90 %d",
			r.P10EndingMinor, r.MedianEndingMinor, r.P90EndingMinor)
	}
}

// A "0% chance of success" for a plan nobody described is the most alarming
// possible way to say "we could not compute this".
func TestRefusalsRatherThanAlarmingZeroes(t *testing.T) {
	cases := []struct {
		name  string
		start int64
		spend int64
		years int
		cfg   Config
	}{
		{"no starting balance", 0, 4_000_000, 30, Config{MeanReturnPct: 7}},
		{"negative starting balance", -1, 4_000_000, 30, Config{MeanReturnPct: 7}},
		{"negative spending", 100_000_000, -1, 30, Config{MeanReturnPct: 7}},
		{"no horizon", 100_000_000, 4_000_000, 0, Config{MeanReturnPct: 7}},
		{"absurd horizon", 100_000_000, 4_000_000, MaxYears + 1, Config{MeanReturnPct: 7}},
		{"too many iterations", 100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7, Iterations: MaxIterations + 1}},
		{"negative iterations", 100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7, Iterations: -5}},
		{"negative volatility", 100_000_000, 4_000_000, 30, Config{MeanReturnPct: 7, StdDevPct: -1}},
		{"impossible return", 100_000_000, 4_000_000, 30, Config{MeanReturnPct: -100}},
	}
	for _, c := range cases {
		if _, ok := Run(c.start, c.spend, c.years, c.cfg); ok {
			t.Errorf("%s: expected a refusal", c.name)
		}
	}
}

func TestTheGeneratorProducesRoughlyTheRequestedDistribution(t *testing.T) {
	// Not a statistics test — a check that the generator is not stuck, biased, or
	// returning the mean every time, any of which would make every success rate
	// meaningless while looking perfectly plausible.
	p := newPCG(12345)
	const n = 20000
	var sum, sumSq float64
	for range n {
		v := p.normal(0.07, 0.15)
		sum += v
		sumSq += v * v
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	sd := math.Sqrt(variance)
	if math.Abs(mean-0.07) > 0.01 {
		t.Errorf("mean = %v, want about 0.07", mean)
	}
	if math.Abs(sd-0.15) > 0.01 {
		t.Errorf("standard deviation = %v, want about 0.15", sd)
	}
}

func TestTheGeneratorDoesNotRepeatItself(t *testing.T) {
	p := newPCG(1)
	seen := map[uint32]bool{}
	for range 1000 {
		v := p.next()
		if seen[v] {
			t.Fatalf("value %d repeated within 1000 draws — the stream is degenerate", v)
		}
		seen[v] = true
	}
}

// The trap this flag exists for: zero is a real standard deviation and "unset"
// is not, and spelling both `StdDevPct: 0` made a deterministic run silently
// come back as a 15% one.
func TestAZeroStdDevMeansUnsetAndNoVolatilityMeansZero(t *testing.T) {
	unset, ok := Run(100_000_000, 5_000_000, 20, Config{MeanReturnPct: 7, StdDevPct: 0, Iterations: 200})
	if !ok {
		t.Fatal("expected a result")
	}
	if unset.P10EndingMinor == unset.P90EndingMinor {
		t.Error("StdDevPct: 0 must fall back to the default spread, not run deterministically")
	}
	flat, ok := Run(100_000_000, 5_000_000, 20, Config{MeanReturnPct: 7, NoVolatility: true, Iterations: 200})
	if !ok {
		t.Fatal("expected a result")
	}
	if flat.P10EndingMinor != flat.P90EndingMinor {
		t.Error("NoVolatility must make every future identical")
	}
}
