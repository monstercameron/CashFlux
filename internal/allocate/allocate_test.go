package allocate

import "testing"

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

func TestScoreNormalizesAndWeights(t *testing.T) {
	c := Candidate{ExpectedReturnAPR: 30, StabilityScore: 50, LiquidityScore: 100}
	// returns caps at 1.0 (30 >= returnsCap), stability 0.5, liquidity 1.0.
	_, b := Score(c, Weights{Returns: 1})
	if !approx(b.Returns, 1) || !approx(b.Stability, 0.5) || !approx(b.Liquidity, 1) {
		t.Errorf("breakdown = %+v, want returns 1, stability .5, liquidity 1", b)
	}
	// Returns-only weighting → score equals the returns breakdown.
	s, _ := Score(c, Weights{Returns: 2})
	if !approx(s, 1) {
		t.Errorf("returns-only score = %g, want 1", s)
	}
}

func TestScoreEqualWeights(t *testing.T) {
	c := Candidate{ExpectedReturnAPR: 0, StabilityScore: 100, LiquidityScore: 0, DebtReduction: true}
	// Equal weights over [0, 1, 0, 1] → 0.5.
	s, _ := Score(c, Weights{Returns: 1, Stability: 1, Liquidity: 1, DebtReduction: 1})
	if !approx(s, 0.5) {
		t.Errorf("score = %g, want 0.5", s)
	}
}

func TestScoreZeroWeightsSafe(t *testing.T) {
	if s, _ := Score(Candidate{StabilityScore: 100}, Weights{}); s < 0 || s > 1 {
		t.Errorf("score with zero weights = %g, want in [0,1]", s)
	}
}

func TestRankOrdersByScore(t *testing.T) {
	cands := []Candidate{
		{ID: "low", ExpectedReturnAPR: 2},
		{ID: "high", ExpectedReturnAPR: 12},
		{ID: "mid", ExpectedReturnAPR: 6},
	}
	ranked := Rank(cands, Weights{Returns: 1})
	want := []string{"high", "mid", "low"}
	for i, w := range want {
		if ranked[i].Candidate.ID != w {
			t.Errorf("rank[%d] = %s, want %s", i, ranked[i].Candidate.ID, w)
		}
	}
}

func TestRankDebtPriority(t *testing.T) {
	cands := []Candidate{
		{ID: "savings", ExpectedReturnAPR: 4, StabilityScore: 90, LiquidityScore: 90},
		{ID: "card", DebtReduction: true},
	}
	// Debt-reduction-weighted profile puts the card first.
	ranked := Rank(cands, Weights{DebtReduction: 5, Returns: 1})
	if ranked[0].Candidate.ID != "card" {
		t.Errorf("debt-weighted rank[0] = %s, want card", ranked[0].Candidate.ID)
	}
}
