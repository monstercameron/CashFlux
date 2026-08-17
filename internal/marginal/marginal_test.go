// SPDX-License-Identifier: MIT

package marginal

import "testing"

func debt(id string, apr float64, owed int64) Destination {
	return Destination{ID: id, Name: id, Kind: KindDebt, AnnualRatePct: apr, CapacityMinor: owed}
}

func savings(id string, rate float64) Destination {
	return Destination{ID: id, Name: id, Kind: KindSavings, AnnualRatePct: rate}
}

func other(id string) Destination {
	return Destination{ID: id, Name: id, Kind: KindOther}
}

// The whole point: "rank 1 and rank 4" is not a comparison anybody can check.
// $220 a year against $40 a year is.
func TestBenefitIsMoneyPerYear(t *testing.T) {
	b := Compute(debt("card", 22, 500_000), 100_000)
	if !b.Known {
		t.Fatal("a debt with an APR must produce a figure")
	}
	if b.AnnualMinor != 22_000 {
		t.Errorf("annual = %d, want 22000 (22%% of $1,000)", b.AnnualMinor)
	}
	s := Compute(savings("hysa", 4), 100_000)
	if s.AnnualMinor != 4_000 {
		t.Errorf("savings annual = %d, want 4000", s.AnnualMinor)
	}
}

// "We do not know what this card charges" and "this card charges nothing" are
// different facts, and only one of them is ever true.
func TestNoRateIsUnknownNotZero(t *testing.T) {
	b := Compute(Destination{ID: "x", Kind: KindDebt}, 100_000)
	if b.Known {
		t.Error("a destination with no rate must not report a computed benefit")
	}
	if b.AnnualMinor != 0 {
		t.Errorf("annual = %d, want the zero value alongside Known=false", b.AnnualMinor)
	}
}

// An emergency reserve has real value that is not interest. Attaching a dollar
// figure would be inventing a number to make a table look complete.
func TestDestinationsWithoutRatesGetNoInventedFigure(t *testing.T) {
	b := Compute(other("emergency"), 500_000)
	if b.Known || b.AnnualMinor != 0 {
		t.Errorf("an 'other' destination produced a benefit: %+v", b)
	}
}

func TestCapacityCapsWhatADestinationCanAbsorb(t *testing.T) {
	// Throwing $5,000 at a card that owes $1,000 only avoids interest on $1,000.
	b := Compute(debt("card", 20, 100_000), 500_000)
	if !b.Capped {
		t.Error("a destination that cannot absorb the amount must say so")
	}
	if b.EffectiveMinor() != 100_000 {
		t.Errorf("effective = %d, want the 100000 capacity", b.EffectiveMinor())
	}
	if b.AnnualMinor != 20_000 {
		t.Errorf("annual = %d, want 20%% of the capacity, not of the offer", b.AnnualMinor)
	}
}

func TestNoCapacityMeansNoLimit(t *testing.T) {
	b := Compute(savings("hysa", 5), 1_000_000)
	if b.Capped {
		t.Error("a destination with no stated capacity must not be treated as full")
	}
	if b.AnnualMinor != 50_000 {
		t.Errorf("annual = %d, want 50000", b.AnnualMinor)
	}
}

func TestCompareOrdersByBenefitWithUnknownsLast(t *testing.T) {
	got := Compare([]Destination{
		savings("hysa", 4),
		other("emergency"),
		debt("card", 22, 900_000),
		debt("loan", 7, 900_000),
	}, 100_000)
	if len(got) != 4 {
		t.Fatalf("results = %d, want 4", len(got))
	}
	if got[0].Destination.ID != "card" || got[1].Destination.ID != "loan" || got[2].Destination.ID != "hysa" {
		t.Errorf("order = %s/%s/%s, want card/loan/hysa",
			got[0].Destination.ID, got[1].Destination.ID, got[2].Destination.ID)
	}
	if got[3].Destination.ID != "emergency" || got[3].Known {
		t.Errorf("last = %+v, want the unknown-benefit destination", got[3].Destination)
	}
}

// Dropping them would quietly remove the emergency fund from a list of places to
// put money — a recommendation dressed as a filter.
func TestDestinationsWithNoFigureAreStillListed(t *testing.T) {
	got := Compare([]Destination{other("emergency"), other("goal")}, 100_000)
	if len(got) != 2 {
		t.Errorf("results = %d, want both listed even with no figures", len(got))
	}
}

func TestCompareIsStableOnTies(t *testing.T) {
	for i := range 5 {
		got := Compare([]Destination{savings("z", 5), savings("a", 5)}, 100_000)
		if got[0].Destination.ID != "a" {
			t.Fatalf("run %d put %q first, want a stable \"a\"", i, got[0].Destination.ID)
		}
	}
}

func TestBestKnownSkipsTheUnmeasurable(t *testing.T) {
	got := Compare([]Destination{other("emergency"), savings("hysa", 4)}, 100_000)
	best, ok := BestKnown(got)
	if !ok || best.Destination.ID != "hysa" {
		t.Errorf("best = %+v (ok=%v), want hysa", best.Destination, ok)
	}
}

func TestBestKnownRefusesWhenNothingIsMeasurable(t *testing.T) {
	// The top of a list sorted by an absent value is not a recommendation.
	if _, ok := BestKnown(Compare([]Destination{other("a"), other("b")}, 100_000)); ok {
		t.Error("nothing measurable must not produce a best destination")
	}
}

// A $3-a-year gap is a decision not worth making, and presenting it with the
// same confidence as a $300 gap wastes somebody's attention.
func TestSpreadSaysWhetherTheChoiceMatters(t *testing.T) {
	wide := Compare([]Destination{debt("card", 22, 900_000), savings("hysa", 1)}, 100_000)
	gap, ok := SpreadMinor(wide)
	if !ok || gap != 21_000 {
		t.Errorf("spread = %d (ok=%v), want 21000", gap, ok)
	}
	narrow := Compare([]Destination{savings("a", 4.0), savings("b", 3.9)}, 100_000)
	gap2, ok2 := SpreadMinor(narrow)
	if !ok2 || gap2 >= 1_000 {
		t.Errorf("narrow spread = %d (ok=%v), want a small gap", gap2, ok2)
	}
}

func TestSpreadRefusesWithFewerThanTwoFigures(t *testing.T) {
	if _, ok := SpreadMinor(Compare([]Destination{savings("only", 4), other("x")}, 100_000)); ok {
		t.Error("one measurable destination has nothing to be compared against")
	}
}

func TestLockLeavesTheRest(t *testing.T) {
	rest, ok := Lock(500_000, 200_000)
	if !ok || rest != 300_000 {
		t.Errorf("remaining = %d (ok=%v), want 300000", rest, ok)
	}
}

// A plan that allocates money nobody has is not a plan, and silently trimming
// the lock would answer a question the user did not ask.
func TestLockRefusesMoreThanThePot(t *testing.T) {
	if _, ok := Lock(100_000, 200_000); ok {
		t.Error("locking more than the total must refuse")
	}
	if _, ok := Lock(100_000, -1); ok {
		t.Error("a negative lock must refuse")
	}
}

func TestZeroAmountProducesNoBenefit(t *testing.T) {
	b := Compute(debt("card", 22, 500_000), 0)
	if b.Known || b.AnnualMinor != 0 {
		t.Errorf("allocating nothing produced a benefit: %+v", b)
	}
}
