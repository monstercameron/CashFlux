// SPDX-License-Identifier: MIT

package portfolio

import (
	"math"
	"testing"
)

// h builds a holding worth `value` minor units at one share.
func h(name, class, sector, region string, value int64, bps int) Holding {
	return Holding{
		Name: name, AssetClass: class, Sector: sector, Region: region,
		Shares: 1, CurrentPriceMinorPerShare: value, ExpenseRatioBps: bps,
	}
}

func weightOf(ws []Weight, label string) (Weight, bool) {
	for _, w := range ws {
		if w.Label == label {
			return w, true
		}
	}
	return Weight{}, false
}

// ─── C377: sector + geography ────────────────────────────────────────────────

// TestUnclassifiedIsItsOwnBucket is the point of the dimension.
//
// Nothing fills sector or region in automatically — the app has no market-data
// feed — so most holdings will have neither for a long time. A view that hid the
// unset ones would show a portfolio with two labelled positions as 100%
// classified, which is a confident answer to a question nobody has answered.
func TestUnclassifiedIsItsOwnBucket(t *testing.T) {
	hs := []Holding{
		h("VTI", "Stocks", "Broad market", "US", 60_000, 0),
		h("Mystery fund", "Stocks", "", "", 40_000, 0),
	}
	sec := AllocationBySector(hs)
	got, ok := weightOf(sec, Unclassified)
	if !ok {
		t.Fatalf("no unclassified bucket in %+v", sec)
	}
	if got.ValueMinor != 40_000 || math.Abs(got.Pct-40) > 0.01 {
		t.Errorf("unclassified = %d (%.1f%%), want 40000 (40%%)", got.ValueMinor, got.Pct)
	}
	// And it is NOT the same label as "other", which means classified-but-doesn't-fit.
	if _, isOther := weightOf(sec, "other"); isOther {
		t.Error("an unset sector landed in \"other\" — that label means something different")
	}
	if reg := AllocationByRegion(hs); len(reg) != 2 {
		t.Errorf("region buckets = %d, want 2", len(reg))
	}
}

func TestAllocationDimensionsSumToTheWhole(t *testing.T) {
	hs := []Holding{
		h("A", "Stocks", "Tech", "US", 50_000, 0),
		h("B", "Bonds", "Govt", "Intl", 30_000, 0),
		h("C", "Stocks", "Tech", "Intl", 20_000, 0),
	}
	for name, ws := range map[string][]Weight{
		"sector": AllocationBySector(hs),
		"region": AllocationByRegion(hs),
		"class":  AllocationByAssetClass(hs),
	} {
		var pct float64
		var val int64
		for _, w := range ws {
			pct += w.Pct
			val += w.ValueMinor
		}
		if math.Abs(pct-100) > 0.01 {
			t.Errorf("%s weights sum to %.2f%%, want 100", name, pct)
		}
		if val != 100_000 {
			t.Errorf("%s values sum to %d, want 100000", name, val)
		}
	}
}

func TestAllocationOfNothingIsNothing(t *testing.T) {
	if AllocationBySector(nil) != nil || AllocationByRegion(nil) != nil {
		t.Error("an empty portfolio produced allocation buckets")
	}
}

// ─── C378: fee drag ──────────────────────────────────────────────────────────

func TestFeesCostAtCurrentValue(t *testing.T) {
	hs := []Holding{
		h("Cheap index", "Stocks", "", "", 100_000_00, 3), // $100k at 0.03% → $30/yr
		h("Costly fund", "Stocks", "", "", 50_000_00, 75), // $50k at 0.75% → $375/yr
	}
	f := Fees(hs)
	if !f.Known() {
		t.Fatal("Known() = false with two ratios recorded")
	}
	if f.AnnualMinor != 30_00+375_00 {
		t.Errorf("AnnualMinor = %d, want %d", f.AnnualMinor, 30_00+375_00)
	}
	// Value-weighted, not a plain mean: (100k×3 + 50k×75) / 150k = 27 bps.
	if math.Abs(f.WeightedBps-27) > 0.01 {
		t.Errorf("WeightedBps = %.2f, want 27 — a plain average would say 39", f.WeightedBps)
	}
	if !f.Complete() {
		t.Error("Complete() = false when every holding carries a ratio")
	}
}

// A fund with no ratio recorded is UNKNOWN, not free. Averaging it in as zero
// would quietly understate the number the household is trying to judge, so it
// is excluded from both the cost and the weighting — and the coverage figure
// says how much of the portfolio the answer actually speaks for.
func TestFeesExcludeUnknownRatiosAndSayHowMuchTheyCover(t *testing.T) {
	hs := []Holding{
		h("Known", "Stocks", "", "", 25_000_00, 40), // $25k at 0.40% → $100/yr
		h("Unknown", "Stocks", "", "", 75_000_00, 0),
	}
	f := Fees(hs)
	if f.AnnualMinor != 100_00 {
		t.Errorf("AnnualMinor = %d, want %d", f.AnnualMinor, 100_00)
	}
	if math.Abs(f.WeightedBps-40) > 0.01 {
		t.Errorf("WeightedBps = %.2f, want 40 — the unknown holding must not average in "+
			"as zero and halve the figure", f.WeightedBps)
	}
	if f.Complete() {
		t.Error("Complete() = true with three quarters of the portfolio unrated")
	}
	if math.Abs(f.CoveragePct()-25) > 0.01 {
		t.Errorf("CoveragePct = %.1f, want 25", f.CoveragePct())
	}
}

// With no ratios at all the surface must say nothing rather than claim the
// portfolio is free.
func TestFeesOfNothingKnownIsNotZeroCost(t *testing.T) {
	f := Fees([]Holding{h("A", "Stocks", "", "", 10_000_00, 0)})
	if f.Known() {
		t.Error("Known() = true with no ratio recorded anywhere")
	}
	if f.AnnualMinor != 0 || f.WeightedBps != 0 {
		t.Errorf("invented a cost: %+v", f)
	}
}

// ─── C379: rebalancing drift ─────────────────────────────────────────────────

func TestRebalanceMeasuresDriftAndDirection(t *testing.T) {
	hs := []Holding{
		h("Equities", "Stocks", "", "", 80_000_00, 0),
		h("Bonds", "Bonds", "", "", 20_000_00, 0),
	}
	targets := []Target{{AssetClass: "Stocks", Pct: 60}, {AssetClass: "Bonds", Pct: 40}}
	if !TargetsValid(targets) {
		t.Fatal("a 60/40 target set was rejected")
	}
	p := Rebalance(hs, targets)

	byClass := map[string]Drift{}
	for _, d := range p.Drifts {
		byClass[d.AssetClass] = d
	}
	stocks, bonds := byClass["Stocks"], byClass["Bonds"]
	if math.Abs(stocks.DriftPct-20) > 0.01 {
		t.Errorf("stocks drift = %.2f, want +20", stocks.DriftPct)
	}
	if !stocks.Overweight() {
		t.Error("stocks at 80%% against a 60%% target is not reported overweight")
	}
	if stocks.DeltaMinor != -20_000_00 {
		t.Errorf("stocks delta = %d, want -2000000 (money moves OUT)", stocks.DeltaMinor)
	}
	if bonds.DeltaMinor != 20_000_00 {
		t.Errorf("bonds delta = %d, want +2000000 (money moves IN)", bonds.DeltaMinor)
	}
	// Counted once: every dollar leaving stocks arrives in bonds, and adding both
	// sides would say twice as much money moves as actually does.
	if p.TotalMinor != 20_000_00 {
		t.Errorf("TotalMinor = %d, want 2000000", p.TotalMinor)
	}
	if math.Abs(p.MaxDriftPct-20) > 0.01 {
		t.Errorf("MaxDriftPct = %.2f, want 20", p.MaxDriftPct)
	}
	if p.Balanced(5) {
		t.Error("Balanced(5%%) = true at 20%% drift")
	}
}

// A class held but never planned for is pure overweight — the case where drift
// matters most, and the one a targets-only loop would silently drop.
func TestRebalanceSurfacesUnplannedHoldings(t *testing.T) {
	hs := []Holding{
		h("Equities", "Stocks", "", "", 90_000_00, 0),
		h("Coins", "Crypto", "", "", 10_000_00, 0),
	}
	p := Rebalance(hs, []Target{{AssetClass: "Stocks", Pct: 100}})
	var found bool
	for _, d := range p.Drifts {
		if d.AssetClass != "Crypto" {
			continue
		}
		found = true
		if d.TargetPct != 0 || !d.Overweight() {
			t.Errorf("crypto drift = %+v, want target 0 and overweight", d)
		}
	}
	if !found {
		t.Fatal("a holding with no target vanished from the drift view")
	}
}

func TestTargetsValidTolerAtesThirds(t *testing.T) {
	if !TargetsValid([]Target{
		{AssetClass: "A", Pct: 33.3}, {AssetClass: "B", Pct: 33.3}, {AssetClass: "C", Pct: 33.4},
	}) {
		t.Error("a set entered as thirds was rejected — the first thing anyone tries")
	}
	for name, ts := range map[string][]Target{
		"empty":     nil,
		"short":     {{AssetClass: "A", Pct: 50}},
		"negative":  {{AssetClass: "A", Pct: -10}, {AssetClass: "B", Pct: 110}},
		"duplicate": {{AssetClass: "A", Pct: 50}, {AssetClass: "A", Pct: 50}},
		"unnamed":   {{AssetClass: "", Pct: 100}},
	} {
		if TargetsValid(ts) {
			t.Errorf("%s target set was accepted", name)
		}
	}
}
