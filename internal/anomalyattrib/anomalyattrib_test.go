// SPDX-License-Identifier: MIT

package anomalyattrib

import "testing"

func p(id string, minor int64) Purchase { return Purchase{ID: id, Payee: id, Minor: minor} }

func TestAFewBigPurchasesExplainAnOverspend(t *testing.T) {
	// $135 over, and one $100 purchase accounts for most of it.
	a := Explain(13_500, []Purchase{
		p("big", 10_000), p("a", 1_200), p("b", 900), p("c", 800),
	})
	if !a.Known {
		t.Fatal("expected an attribution")
	}
	if !a.Concentrated {
		t.Errorf("explained %.0f%%, want it treated as concentrated", a.ExplainedPct)
	}
	if len(a.Culprits) != 1 || a.Culprits[0].ID != "big" {
		t.Errorf("culprits = %+v, want just the big one", a.Culprits)
	}
	if a.ExplainedPct < 70 || a.ExplainedPct > 76 {
		t.Errorf("explained = %.1f%%, want about 74", a.ExplainedPct)
	}
}

// One big purchase is a one-off; thirty small ones are a habit. A tool that
// cannot tell them apart sends somebody hunting for a culprit that does not
// exist.
func TestDiffuseSpendingNamesNoCulprit(t *testing.T) {
	var many []Purchase
	for i := range 30 {
		many = append(many, p(string(rune('a'+i)), 500))
	}
	a := Explain(13_500, many)
	if !a.Known {
		t.Fatal("expected an attribution")
	}
	if a.Concentrated {
		t.Errorf("thirty equal purchases were called concentrated (%.0f%%)", a.ExplainedPct)
	}
	if len(a.Culprits) != 0 {
		t.Errorf("culprits = %+v, want none — a weak list reads as a strong claim", a.Culprits)
	}
}

func TestItStopsAsSoonAsEnoughIsExplained(t *testing.T) {
	// Two purchases cross the threshold, so a third is not named — the point is a
	// shortcut, not a list.
	a := Explain(10_000, []Purchase{
		p("x", 4_000), p("y", 3_000), p("z", 500), p("w", 400), p("v", 300),
	})
	if len(a.Culprits) != 2 {
		t.Errorf("culprits = %d, want 2 — enough to cross the threshold and no more", len(a.Culprits))
	}
}

func TestEqualPurchasesAreDiffuseHoweverManyItTakes(t *testing.T) {
	// Six identical dinners "explaining 67%" is arithmetic, not a culprit —
	// reporting it as a cause would point at four ordinary meals. Concentration
	// has to mean FEW purchases explaining most, not a cumulative share that any
	// equal set reaches given enough of them.
	var six []Purchase
	for i := range 6 {
		six = append(six, p(string(rune('a'+i)), 1_000))
	}
	a := Explain(6_000, six)
	if a.Concentrated {
		t.Errorf("six equal purchases were called concentrated (%.0f%%)", a.ExplainedPct)
	}
}

func TestUnusuallyLargePurchasesAreCounted(t *testing.T) {
	a := Explain(10_000, []Purchase{
		{ID: "x", Payee: "Restaurant", Minor: 9_000, Larger: true},
		{ID: "y", Payee: "Cafe", Minor: 500},
	})
	if !a.Concentrated {
		t.Fatalf("explained %.0f%%, want concentrated", a.ExplainedPct)
	}
	if a.UnusuallyLarge != 1 {
		t.Errorf("unusually large = %d, want 1", a.UnusuallyLarge)
	}
}

// "0% explained" reads as a failed analysis of a real problem, when in fact
// there was no problem.
func TestNoOverspendIsNotAFailedAnalysis(t *testing.T) {
	if a := Explain(0, []Purchase{p("x", 500)}); a.Known {
		t.Errorf("no overspend produced an attribution: %+v", a)
	}
	if a := Explain(-500, []Purchase{p("x", 500)}); a.Known {
		t.Error("an underspend produced an attribution")
	}
}

func TestNoPurchasesIsAlsoUnknown(t *testing.T) {
	if a := Explain(10_000, nil); a.Known {
		t.Error("an overspend with nothing to attribute it to produced an attribution")
	}
}

func TestTheThresholdEdge(t *testing.T) {
	// Exactly at the threshold counts as explained.
	a := Explain(10_000, []Purchase{p("x", 6_000), p("y", 100)})
	if !a.Concentrated {
		t.Errorf("exactly %v%% was not treated as explained (%.1f%%)", ExplainedPct, a.ExplainedPct)
	}
	// Just below does not.
	b := Explain(10_000, []Purchase{p("x", 5_900), p("y", 10), p("z", 10), p("w", 10), p("v", 10)})
	if b.Concentrated {
		t.Errorf("%.1f%% was treated as explained", b.ExplainedPct)
	}
}

func TestAttributionIsStableAcrossRuns(t *testing.T) {
	// The same period must attribute identically, or a finding changes its story
	// between renders.
	ps := []Purchase{p("z", 800), p("a", 8_000), p("m", 100), p("q", 100), p("r", 100), p("s", 100)}
	for i := range 5 {
		a := Explain(10_000, ps)
		if len(a.Culprits) < 1 || a.Culprits[0].ID != "a" {
			t.Fatalf("run %d attributed to %+v", i, a.Culprits)
		}
	}
}

func TestTotalMinorSums(t *testing.T) {
	if got := TotalMinor([]Purchase{p("a", 100), p("b", 250)}); got != 350 {
		t.Errorf("total = %d, want 350", got)
	}
}

// "1 purchase explains 280% of it" reads as a broken calculation, because it is
// one — the culprits are measured against the OVERAGE, and a single receipt can
// easily exceed it.
func TestNothingExplainsMoreThanAllOfIt(t *testing.T) {
	a := Explain(15_002, []Purchase{p("one", 42_000)})
	if a.ExplainedPct > 100 {
		t.Errorf("explained = %.0f%%, want it capped at 100", a.ExplainedPct)
	}
	if !a.Everything {
		t.Error("a purchase covering the whole overspend did not report Everything")
	}
	if !a.Concentrated {
		t.Error("one purchase covering the whole overspend was not concentrated")
	}
	partial := Explain(10_000, []Purchase{p("x", 7_000), p("y", 100), p("z", 100)})
	if partial.Everything {
		t.Error("a partial explanation reported that it accounted for everything")
	}
}
