// SPDX-License-Identifier: MIT

package healthscore

import (
	"math"
	"testing"
)

// full is a healthy household with all five factors applicable, used as a base.
func full() Inputs {
	return Inputs{
		SavingsRatePct: 20, HasIncome: true,
		EmergencyMonths: 6, HasLiquidData: true,
		ObligationRatioPct: 10, HasLiabilities: true,
		BudgetAdherencePct: 100, HasBudgets: true,
		AggUtilizationPct: 5, HasCredit: true,
	}
}

func TestEvaluate_AllPerfect(t *testing.T) {
	r := Evaluate(full())
	if r.Score != 100 {
		t.Fatalf("perfect inputs: score=%d, want 100", r.Score)
	}
	if r.Band != BandExcellent {
		t.Errorf("band=%q, want Excellent", r.Band)
	}
	if r.ApplicableCount != 5 {
		t.Errorf("applicable=%d, want 5", r.ApplicableCount)
	}
	if len(r.Steps) != 0 {
		t.Errorf("perfect score should have no steps, got %d", len(r.Steps))
	}
}

func TestFactorScoreCurves(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"savings 0%", savingsScore(0), 0},
		{"savings negative", savingsScore(-30), 0},
		{"savings 10%", savingsScore(10), 50},
		{"savings 20%+", savingsScore(40), 100},
		{"emergency 0mo", emergencyScore(0), 0},
		{"emergency 1mo", emergencyScore(1), 25},
		{"emergency 3mo", emergencyScore(3), 60},
		{"emergency 6mo", emergencyScore(6), 100},
		{"emergency 9mo", emergencyScore(9), 100},
		{"debt 10%", obligationScore(10, true), 100},
		{"debt 36%", obligationScore(36, true), 50},
		{"debt 43%+", obligationScore(50, true), 0},
		{"debt none", obligationScore(0, false), 100},
		{"util 5%", utilizationScore(5), 100},
		{"util 30%", utilizationScore(30), 70},
		{"util 80%+", utilizationScore(90), 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestNegativeSavings_NoHardCapButFlagged(t *testing.T) {
	in := full()
	in.SavingsRatePct = -50 // overspending; savings factor → 0
	r := Evaluate(in)
	if !r.NegativeCashFlow {
		t.Error("expected NegativeCashFlow=true")
	}
	// The other four factors are perfect, so the score must NOT be hard-capped at <50:
	// only the savings factor's 0 drags the weighted average. Excellent everywhere else
	// must still produce a strong score (proving no double-penalty hard cap).
	if r.Score < 70 {
		t.Errorf("negative savings should only cost the savings factor, score=%d want >=70 (no hard cap)", r.Score)
	}
}

func TestRetiree_NoIncome_DropsSavingsAndDebt(t *testing.T) {
	// A retiree drawing down savings: no income, but emergency + budgets + utilization
	// still yield a valid score (≥2 applicable). Savings and debt factors drop.
	in := Inputs{
		HasIncome:       false,
		EmergencyMonths: 6, HasLiquidData: true,
		BudgetAdherencePct: 100, HasBudgets: true,
		AggUtilizationPct: 5, HasCredit: true,
	}
	r := Evaluate(in)
	if r.Band == BandNoData {
		t.Fatalf("retiree with 3 applicable factors should score, got NoData")
	}
	if r.ApplicableCount != 3 {
		t.Errorf("applicable=%d, want 3", r.ApplicableCount)
	}
	// emergency .25 + budget .15 + utilization .15 = .55 applicable weight; all perfect → 100.
	if r.Score != 100 {
		t.Errorf("score=%d, want 100 (all applicable factors perfect)", r.Score)
	}
	for _, f := range r.Factors {
		if f.Key == "savings" || f.Key == "debt" {
			if f.Applicable {
				t.Errorf("%s should be inapplicable with no income", f.Key)
			}
			if f.ContributionPct != 0 {
				t.Errorf("%s inapplicable should contribute 0, got %d", f.Key, f.ContributionPct)
			}
		}
	}
}

func TestZeroDebt_ScoredNotDropped(t *testing.T) {
	in := full()
	in.HasLiabilities = false
	r := Evaluate(in)
	var debt Factor
	for _, f := range r.Factors {
		if f.Key == "debt" {
			debt = f
		}
	}
	if !debt.Applicable {
		t.Error("zero-debt (with income) must be APPLICABLE, not dropped")
	}
	if debt.Score != 100 {
		t.Errorf("zero debt should score 100, got %d", debt.Score)
	}
	if debt.Value != "no debt" {
		t.Errorf("zero-debt value=%q, want 'no debt'", debt.Value)
	}
	if r.ApplicableCount != 5 {
		t.Errorf("applicable=%d, want 5 (debt still applies)", r.ApplicableCount)
	}
}

func TestNoCards_DropsUtilization(t *testing.T) {
	in := full()
	in.HasCredit = false
	r := Evaluate(in)
	for _, f := range r.Factors {
		if f.Key == "utilization" && f.Applicable {
			t.Error("utilization must be dropped when there are no cards")
		}
	}
	if r.ApplicableCount != 4 {
		t.Errorf("applicable=%d, want 4", r.ApplicableCount)
	}
}

func TestNotEnoughData(t *testing.T) {
	// Only one applicable factor → NoData.
	in := Inputs{BudgetAdherencePct: 100, HasBudgets: true}
	r := Evaluate(in)
	if r.Band != BandNoData {
		t.Errorf("one factor should be NoData, got band=%q score=%d", r.Band, r.Score)
	}
	if r.Score != 0 {
		t.Errorf("NoData score should be 0, got %d", r.Score)
	}
}

// Re-normalization must redistribute a dropped factor's weight PROPORTIONALLY across
// the rest (so applicable weights sum to 1). We verify by checking ContributionPct
// for every single-factor-missing permutation, and that the overall score equals the
// hand-computed weighted average over remaining weights.
func TestReNormalization_EveryPermutation(t *testing.T) {
	base := full()
	// Distinct factor scores so weighting actually matters.
	base.SavingsRatePct = 10  // 50
	base.EmergencyMonths = 3  // 60
	base.ObligationRatioPct = 36 // 50
	base.BudgetAdherencePct = 80 // 80
	base.AggUtilizationPct = 30  // 70
	scores := map[string]float64{"savings": 50, "emergency": 60, "debt": 50, "budget": 80, "utilization": 70}
	weights := map[string]float64{"savings": .25, "emergency": .25, "debt": .20, "budget": .15, "utilization": .15}

	drop := []struct {
		key   string
		apply func(*Inputs)
	}{
		{"savings", func(i *Inputs) { i.HasIncome = false }}, // also drops debt — handle below
		{"emergency", func(i *Inputs) { i.HasLiquidData = false }},
		{"budget", func(i *Inputs) { i.HasBudgets = false }},
		{"utilization", func(i *Inputs) { i.HasCredit = false }},
	}
	for _, d := range drop {
		in := base
		d.apply(&in)
		r := Evaluate(in)
		// Determine which keys remain applicable.
		remaining := map[string]bool{"savings": true, "emergency": true, "debt": true, "budget": true, "utilization": true}
		if !in.HasIncome {
			remaining["savings"] = false
			remaining["debt"] = false
		}
		if !in.HasLiquidData {
			remaining["emergency"] = false
		}
		if !in.HasBudgets {
			remaining["budget"] = false
		}
		if !in.HasCredit {
			remaining["utilization"] = false
		}
		var wsum, weighted float64
		for k, on := range remaining {
			if on {
				wsum += weights[k]
			}
		}
		for k, on := range remaining {
			if on {
				weighted += scores[k] * (weights[k] / wsum)
			}
		}
		want := clampPct(int(math.Round(weighted)))
		if r.Score != want {
			t.Errorf("drop %s: score=%d, want %d (re-normalized over weight sum %.2f)", d.key, r.Score, want, wsum)
		}
		// ContributionPct of applicable factors must sum to ~100.
		var contrib int
		for _, f := range r.Factors {
			if f.Applicable {
				contrib += f.ContributionPct
			}
		}
		if contrib < 98 || contrib > 102 {
			t.Errorf("drop %s: contributions sum to %d, want ~100", d.key, contrib)
		}
	}
}

func TestBands(t *testing.T) {
	cases := []struct {
		score int
		band  Band
	}{
		{95, BandExcellent}, {80, BandExcellent},
		{79, BandGood}, {60, BandGood},
		{59, BandFair}, {40, BandFair},
		{39, BandNeedsWork}, {25, BandNeedsWork},
		{24, BandCritical}, {0, BandCritical},
	}
	for _, c := range cases {
		if got := bandFor(c.score); got != c.band {
			t.Errorf("bandFor(%d)=%q, want %q", c.score, got, c.band)
		}
	}
}

func TestStepsFromWeakestFactors(t *testing.T) {
	in := Inputs{
		SavingsRatePct: 3, HasIncome: true, // ~15 → weak
		EmergencyMonths: 0.1, HasLiquidData: true, // ~3 → weakest
		ObligationRatioPct: 10, HasLiabilities: true, // 100 → strong, no step
		BudgetAdherencePct: 100, HasBudgets: true, // 100 → no step
		AggUtilizationPct: 5, HasCredit: true, // 100 → no step
	}
	r := Evaluate(in)
	if len(r.Steps) == 0 {
		t.Fatal("expected steps for weak factors")
	}
	// Weakest (emergency) should be first.
	if r.Steps[0].Factor != "Emergency fund" {
		t.Errorf("first step=%q, want Emergency fund (weakest)", r.Steps[0].Factor)
	}
	for _, s := range r.Steps {
		if s.Action == "" || s.Target == "" {
			t.Errorf("step %q missing action/target", s.Factor)
		}
	}
}

func TestClampingOutOfRange(t *testing.T) {
	in := Inputs{
		SavingsRatePct: 999, HasIncome: true,
		ObligationRatioPct: 250, HasLiabilities: true, // DTI > 100% → 0
		BudgetAdherencePct: 150, HasBudgets: true,
		AggUtilizationPct: 250, HasCredit: true,
		EmergencyMonths: 50, HasLiquidData: true,
	}
	r := Evaluate(in)
	if r.Score < 0 || r.Score > 100 {
		t.Errorf("score out of range: %d", r.Score)
	}
	for _, f := range r.Factors {
		if f.Score < 0 || f.Score > 100 {
			t.Errorf("factor %s score out of range: %d", f.Key, f.Score)
		}
	}
}
