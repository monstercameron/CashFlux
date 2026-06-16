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

func TestRankWithExcludes(t *testing.T) {
	cands := []Candidate{
		{ID: "a", ExpectedReturnAPR: 10},
		{ID: "b", ExpectedReturnAPR: 5},
		{ID: "c", ExpectedReturnAPR: 8},
	}
	w := Weights{Returns: 1}

	// Excluding "a" leaves b and c, ranked by return (c before b).
	got := RankWith(cands, w, Constraints{Exclude: map[string]bool{"a": true}})
	if len(got) != 2 {
		t.Fatalf("expected 2 ranked, got %d", len(got))
	}
	if got[0].Candidate.ID != "c" || got[1].Candidate.ID != "b" {
		t.Errorf("order = %s,%s; want c,b", got[0].Candidate.ID, got[1].Candidate.ID)
	}
	for _, r := range got {
		if r.Candidate.ID == "a" {
			t.Error("excluded candidate a should not appear")
		}
	}
}

func TestRankWithZeroConstraintsEqualsRank(t *testing.T) {
	cands := []Candidate{
		{ID: "a", ExpectedReturnAPR: 3},
		{ID: "b", ExpectedReturnAPR: 9},
	}
	w := Weights{Returns: 1}
	a := Rank(cands, w)
	b := RankWith(cands, w, Constraints{})
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Candidate.ID != b[i].Candidate.ID || !approx(a[i].Score, b[i].Score) {
			t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestConstraintsEligible(t *testing.T) {
	c := Constraints{Exclude: map[string]bool{"x": true}}
	if c.Eligible(Candidate{ID: "x"}) {
		t.Error("x should be ineligible")
	}
	if !c.Eligible(Candidate{ID: "y"}) {
		t.Error("y should be eligible")
	}
	// Zero-value constraints accept everything.
	if !(Constraints{}).Eligible(Candidate{ID: "x"}) {
		t.Error("zero constraints should accept all")
	}
}

func TestDistributeProportional(t *testing.T) {
	ranked := []Ranked{
		{Candidate: Candidate{ID: "a"}, Score: 0.75},
		{Candidate: Candidate{ID: "b"}, Score: 0.25},
	}
	plans, rem := Distribute(ranked, 1000, SplitOptions{})
	if plans[0].Amount != 750 || plans[1].Amount != 250 {
		t.Errorf("amounts = %d,%d; want 750,250", plans[0].Amount, plans[1].Amount)
	}
	if rem != 0 {
		t.Errorf("remainder = %d; want 0", rem)
	}
}

func TestDistributeReserve(t *testing.T) {
	ranked := []Ranked{
		{Candidate: Candidate{ID: "a"}, Score: 0.75},
		{Candidate: Candidate{ID: "b"}, Score: 0.25},
	}
	plans, rem := Distribute(ranked, 1000, SplitOptions{Reserve: 200})
	// available 800 → 600 / 200; remainder keeps the 200 reserve.
	if plans[0].Amount != 600 || plans[1].Amount != 200 {
		t.Errorf("amounts = %d,%d; want 600,200", plans[0].Amount, plans[1].Amount)
	}
	if rem != 200 {
		t.Errorf("remainder = %d; want 200", rem)
	}
}

func TestDistributeMaxPerCap(t *testing.T) {
	ranked := []Ranked{
		{Candidate: Candidate{ID: "a"}, Score: 0.75},
		{Candidate: Candidate{ID: "b"}, Score: 0.25},
	}
	plans, rem := Distribute(ranked, 1000, SplitOptions{MaxPer: 500})
	// a would get 750 but is capped at 500; b gets 250. 250 stays unallocated.
	if plans[0].Amount != 500 || plans[1].Amount != 250 {
		t.Errorf("amounts = %d,%d; want 500,250", plans[0].Amount, plans[1].Amount)
	}
	if rem != 250 {
		t.Errorf("remainder = %d; want 250", rem)
	}
}

func TestDistributeEqualWhenNoScores(t *testing.T) {
	ranked := []Ranked{
		{Candidate: Candidate{ID: "a"}},
		{Candidate: Candidate{ID: "b"}},
	}
	plans, rem := Distribute(ranked, 1000, SplitOptions{})
	if plans[0].Amount != 500 || plans[1].Amount != 500 || rem != 0 {
		t.Errorf("even split failed: %d,%d rem %d", plans[0].Amount, plans[1].Amount, rem)
	}
}

func TestDistributeEdgeCases(t *testing.T) {
	// No candidates → whole total is remainder.
	if plans, rem := Distribute(nil, 1000, SplitOptions{}); len(plans) != 0 || rem != 1000 {
		t.Errorf("empty: plans %d rem %d; want 0,1000", len(plans), rem)
	}
	// Reserve exceeds total → nothing allocated, total is remainder.
	ranked := []Ranked{{Candidate: Candidate{ID: "a"}, Score: 1}}
	plans, rem := Distribute(ranked, 100, SplitOptions{Reserve: 500})
	if plans[0].Amount != 0 || rem != 100 {
		t.Errorf("over-reserve: amount %d rem %d; want 0,100", plans[0].Amount, rem)
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
