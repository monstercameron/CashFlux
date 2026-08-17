// SPDX-License-Identifier: MIT

package retirement

import (
	"math"
	"testing"
)

var std = Assumptions{ReturnPct: 7, InflationPct: 3}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// The Fisher relation, not a subtraction. At 7% and 3% that is 3.88% versus
// 4.00% — small annually, roughly a 4% error in the final balance over thirty
// years, which is a whole year of contributions.
func TestRealReturnUsesFisherNotSubtraction(t *testing.T) {
	got := std.RealReturnPct()
	if !near(got, 3.883495, 0.0001) {
		t.Errorf("RealReturnPct = %v, want ~3.8835", got)
	}
	if near(got, 4.0, 0.001) {
		t.Error("RealReturnPct is a plain subtraction")
	}
}

// A contribution made through the year has, on average, been invested half of
// it. Crediting a full year's growth to money that arrived in December is the
// commonest way a retirement projection flatters itself.
func TestContributionsLandAtYearEnd(t *testing.T) {
	// One year, no starting balance, $10,000 in: the balance must be exactly the
	// contribution, with no growth applied to it.
	p := Project(0, 1000000, 1, Assumptions{ReturnPct: 7})
	if !p.Known {
		t.Fatal("unknown")
	}
	if p.FinalNominalMinor != 1000000 {
		t.Errorf("final = %d, want exactly the contribution (no growth on year-end money)",
			p.FinalNominalMinor)
	}
}

func TestProjectCompounds(t *testing.T) {
	// $100,000 at 7% for one year, no contributions.
	p := Project(10000000, 0, 1, Assumptions{ReturnPct: 7})
	if p.FinalNominalMinor != 10700000 {
		t.Errorf("one year at 7%% = %d, want 10700000", p.FinalNominalMinor)
	}
	// Ten years compounding, not ten years of simple interest.
	long := Project(10000000, 0, 10, Assumptions{ReturnPct: 7})
	simple := int64(10000000 * (1 + 0.07*10))
	if long.FinalNominalMinor <= simple {
		t.Errorf("ten years = %d, which is not better than simple interest %d", long.FinalNominalMinor, simple)
	}
}

// A nominal projection of $2.1M in 2056 means nothing to a person — they cannot
// price 2056 groceries. Both figures are carried, and the real one is smaller
// whenever there is inflation.
func TestRealIsDiscountedAgainstNominal(t *testing.T) {
	p := Project(10000000, 500000, 30, std)
	if !p.Known {
		t.Fatal("unknown")
	}
	if p.FinalRealMinor >= p.FinalNominalMinor {
		t.Errorf("real %d is not below nominal %d under 3%% inflation",
			p.FinalRealMinor, p.FinalNominalMinor)
	}
	// With no inflation the two must agree exactly — otherwise the discounting is
	// being applied where it should not be.
	flat := Project(10000000, 500000, 30, Assumptions{ReturnPct: 7})
	if flat.FinalRealMinor != flat.FinalNominalMinor {
		t.Errorf("with no inflation real %d != nominal %d",
			flat.FinalRealMinor, flat.FinalNominalMinor)
	}
	// Year 0 is today's balance, untouched, in both currencies.
	if p.Years[0].NominalMinor != 10000000 || p.Years[0].RealMinor != 10000000 {
		t.Errorf("year 0 = %+v, want today's balance in both", p.Years[0])
	}
}

// Growth is what the MARKET added, as opposed to what the saver did.
func TestGrowthSeparatesReturnsFromContributions(t *testing.T) {
	p := Project(10000000, 1000000, 10, Assumptions{ReturnPct: 7})
	if p.ContributedMinor != 10000000 {
		t.Errorf("Contributed = %d, want 10 x 1,000,000", p.ContributedMinor)
	}
	if p.GrowthMinor() <= 0 {
		t.Errorf("Growth = %d, want positive at a 7%% return", p.GrowthMinor())
	}
	// start + contributions + growth == final, exactly.
	if got := 10000000 + p.ContributedMinor + p.GrowthMinor(); got != p.FinalNominalMinor {
		t.Errorf("start+contributed+growth = %d but final = %d", got, p.FinalNominalMinor)
	}
}

// Never a zero balance for a bad input — that reads as "you will have nothing".
func TestProjectRefusesRatherThanReturningZero(t *testing.T) {
	for _, c := range []struct {
		name  string
		years int
		a     Assumptions
	}{
		{"no horizon", 0, std},
		{"negative horizon", -5, std},
		{"absurd horizon", MaxYears + 1, std},
		{"impossible return", 10, Assumptions{ReturnPct: -150}},
		{"impossible inflation", 10, Assumptions{InflationPct: -150}},
	} {
		got := Project(10000000, 500000, c.years, c.a)
		if got.Known {
			t.Errorf("%s: reported a known projection", c.name)
		}
		if got.FinalNominalMinor != 0 || len(got.Years) != 0 {
			t.Errorf("%s: produced figures anyway: %+v", c.name, got)
		}
	}
}

// ─── drawdown ────────────────────────────────────────────────────────────────

// Someone withdrawing $60,000 today needs more than $60,000 in ten years to buy
// the same life. A flat nominal withdrawal reports a nest egg lasting far longer
// than it does.
func TestWithdrawalsAreInflationIndexed(t *testing.T) {
	indexed := RunDrawdown(100000000, 6000000, 40, std)
	flat := RunDrawdown(100000000, 6000000, 40, Assumptions{ReturnPct: 7})
	if !indexed.Known || !flat.Known {
		t.Fatal("unknown")
	}
	if !indexed.Depleted {
		t.Fatalf("an inflation-indexed 6%% withdrawal survived 40 years: %+v", indexed.LastsYears)
	}
	if flat.Depleted && flat.LastsYears <= indexed.LastsYears {
		t.Errorf("flat withdrawals lasted %d years, indexed %d — indexing must shorten it",
			flat.LastsYears, indexed.LastsYears)
	}
	// Later withdrawals are larger than earlier ones.
	if indexed.Years[3].WithdrawnMinor <= indexed.Years[1].WithdrawnMinor {
		t.Errorf("withdrawals are not growing: %d then %d",
			indexed.Years[1].WithdrawnMinor, indexed.Years[3].WithdrawnMinor)
	}
}

// Surviving the horizon is the GOOD outcome and must not read as "we did not
// check".
func TestSurvivingTheHorizonIsNotDepletion(t *testing.T) {
	d := RunDrawdown(100000000, 1000000, 30, std)
	if !d.Known {
		t.Fatal("unknown")
	}
	if d.Depleted {
		t.Errorf("a 1%% withdrawal depleted in 30 years: lasted %d", d.LastsYears)
	}
	if d.LastsYears != 30 {
		t.Errorf("LastsYears = %d, want the full horizon", d.LastsYears)
	}
	if d.EndingNominalMinor <= 0 {
		t.Errorf("survived but ended with %d", d.EndingNominalMinor)
	}
}

// The final year takes what is LEFT, not what was wanted — reporting a
// withdrawal the balance could not fund would overstate the last year.
func TestTheFinalYearTakesOnlyWhatRemains(t *testing.T) {
	d := RunDrawdown(1500000, 1000000, 10, Assumptions{})
	if !d.Depleted {
		t.Fatalf("$15,000 against $10,000/yr did not deplete: %+v", d)
	}
	last := d.Years[len(d.Years)-1]
	if last.WithdrawnMinor > 1000000 {
		t.Errorf("the final year withdrew %d, more than the annual amount", last.WithdrawnMinor)
	}
	if last.NominalMinor != 0 {
		t.Errorf("depleted but ended at %d", last.NominalMinor)
	}
	// A year and a half of money lasts one whole year.
	if d.LastsYears != 1 {
		t.Errorf("LastsYears = %d, want 1", d.LastsYears)
	}
}

func TestDrawdownRefusesBadInputs(t *testing.T) {
	for _, c := range []struct {
		name    string
		draw    int64
		horizon int
		a       Assumptions
	}{
		{"no withdrawal", 0, 30, std},
		{"negative withdrawal", -100, 30, std},
		{"no horizon", 100, 0, std},
		{"impossible return", 100, 30, Assumptions{ReturnPct: -150}},
	} {
		if got := RunDrawdown(100000000, c.draw, c.horizon, c.a); got.Known {
			t.Errorf("%s: reported a known drawdown", c.name)
		}
	}
}

// ─── FIRE ────────────────────────────────────────────────────────────────────

func TestFIRENumber(t *testing.T) {
	// $60,000 a year at 4% = $1,500,000.
	got, ok := FIRENumber(6000000, DefaultSWRPct)
	if !ok || got != 150000000 {
		t.Errorf("FIRENumber = %d,%v want 150000000,true", got, ok)
	}
	// A stricter rate needs a bigger pot.
	strict, _ := FIRENumber(6000000, 3)
	if strict <= got {
		t.Errorf("a 3%% rate needs %d, not more than the 4%% figure %d", strict, got)
	}
}

// A zero rate is a division by zero dressed as "never withdraw"; a zero expense
// figure means nobody said what the life costs, and "you need nothing" would be
// a confident absurdity.
func TestFIRENumberRefusesTheDegenerateCases(t *testing.T) {
	if _, ok := FIRENumber(6000000, 0); ok {
		t.Error("a zero withdrawal rate produced a number")
	}
	if _, ok := FIRENumber(0, 4); ok {
		t.Error("zero expenses produced a number")
	}
	if _, ok := FIRENumber(-100, 4); ok {
		t.Error("negative expenses produced a number")
	}
}

func TestYearsToFI(t *testing.T) {
	// Already there.
	if n, ok := YearsToFI(200000000, 0, 150000000, 4); !ok || n != 0 {
		t.Errorf("already-past-target = %d,%v want 0,true", n, ok)
	}
	// Saving with real growth gets there eventually.
	n, ok := YearsToFI(10000000, 3000000, 150000000, 4)
	if !ok || n <= 0 {
		t.Fatalf("YearsToFI = %d,%v", n, ok)
	}
	// More saving gets there sooner.
	faster, _ := YearsToFI(10000000, 6000000, 150000000, 4)
	if faster >= n {
		t.Errorf("doubling contributions took %d years vs %d", faster, n)
	}
}

// "Never" must not be presented as "eventually" via a large number.
func TestYearsToFIRefusesTheUnreachable(t *testing.T) {
	if _, ok := YearsToFI(1000, 0, 150000000, 0); ok {
		t.Error("no contribution and no growth reported a path to the target")
	}
	if _, ok := YearsToFI(0, 100, 1<<62, 1); ok {
		t.Error("an unreachable target inside the horizon reported success")
	}
	if _, ok := YearsToFI(1000, 1000, 0, 4); ok {
		t.Error("a zero target reported success")
	}
}

// The result echoes what it rests on, so a surface can state the assumptions
// without holding the inputs itself.
func TestResultsCarryTheirAssumptions(t *testing.T) {
	p := Project(1000, 100, 5, std)
	if p.Assumptions != std {
		t.Errorf("Projection assumptions = %+v", p.Assumptions)
	}
	d := RunDrawdown(100000, 1000, 5, std)
	if d.Assumptions != std {
		t.Errorf("Drawdown assumptions = %+v", d.Assumptions)
	}
	// Even a refused run echoes them, so an error state can still say what was
	// attempted.
	bad := Project(1000, 100, 0, std)
	if bad.Assumptions != std {
		t.Errorf("a refused projection dropped its assumptions: %+v", bad.Assumptions)
	}
}
