// SPDX-License-Identifier: MIT

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

func TestGoalProgressCriterion(t *testing.T) {
	// The goal-progress breakdown is the clamped completion fraction; non-goal
	// candidates (and out-of-range values) behave sensibly.
	cases := []struct {
		name     string
		progress float64
		want     float64
	}{
		{"unset/non-goal", 0, 0},
		{"half done", 0.5, 0.5},
		{"almost done", 0.9, 0.9},
		{"complete", 1, 1},
		{"over-range clamps", 1.4, 1},
		{"negative clamps", -0.2, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, b := Score(Candidate{GoalProgress: tc.progress}, Weights{GoalProgress: 1})
			if !approx(b.GoalProgress, tc.want) {
				t.Errorf("GoalProgress breakdown = %g, want %g", b.GoalProgress, tc.want)
			}
		})
	}

	// Goal-progress-only weighting ranks the goal nearest completion first.
	cands := []Candidate{
		{ID: "far", GoalProgress: 0.2},
		{ID: "near", GoalProgress: 0.85},
		{ID: "mid", GoalProgress: 0.5},
	}
	got := Rank(cands, Weights{GoalProgress: 1})
	if got[0].Candidate.ID != "near" || got[1].Candidate.ID != "mid" || got[2].Candidate.ID != "far" {
		t.Errorf("order = %s,%s,%s; want near,mid,far",
			got[0].Candidate.ID, got[1].Candidate.ID, got[2].Candidate.ID)
	}
}

func TestGoalProgressZeroWeightDoesNotChangeScore(t *testing.T) {
	// Adding the criterion is backward-compatible: with zero goal-progress weight,
	// a candidate's score is identical regardless of its GoalProgress value.
	w := Weights{Returns: 1, Stability: 1}
	a, _ := Score(Candidate{ExpectedReturnAPR: 9, StabilityScore: 60, GoalProgress: 0}, w)
	b, _ := Score(Candidate{ExpectedReturnAPR: 9, StabilityScore: 60, GoalProgress: 0.9}, w)
	if !approx(a, b) {
		t.Errorf("zero-weighted goal progress changed score: %g vs %g", a, b)
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

func TestRankTieStability(t *testing.T) {
	// All equal returns → equal scores → input order preserved (stable sort).
	cands := []Candidate{
		{ID: "first", ExpectedReturnAPR: 5},
		{ID: "second", ExpectedReturnAPR: 5},
		{ID: "third", ExpectedReturnAPR: 5},
	}
	ranked := Rank(cands, Weights{Returns: 1})
	want := []string{"first", "second", "third"}
	for i, w := range want {
		if ranked[i].Candidate.ID != w {
			t.Errorf("tie order[%d] = %s, want %s (stable)", i, ranked[i].Candidate.ID, w)
		}
	}
}

func TestRankAndDistributeDeterministic(t *testing.T) {
	cands := []Candidate{
		{ID: "a", ExpectedReturnAPR: 7, StabilityScore: 60, LiquidityScore: 80},
		{ID: "b", ExpectedReturnAPR: 12, StabilityScore: 30, LiquidityScore: 40, DebtReduction: true},
		{ID: "c", ExpectedReturnAPR: 4, StabilityScore: 95, LiquidityScore: 95},
	}
	w := Weights{Returns: 2, Stability: 1, Liquidity: 1, DebtReduction: 3}
	first := Rank(cands, w)
	for run := 0; run < 25; run++ {
		got := Rank(cands, w)
		if len(got) != len(first) {
			t.Fatalf("run %d: length changed", run)
		}
		for i := range got {
			if got[i].Candidate.ID != first[i].Candidate.ID || !approx(got[i].Score, first[i].Score) {
				t.Fatalf("run %d index %d: %+v != %+v (non-deterministic)", run, i, got[i], first[i])
			}
		}
		// Distribute over the same ranking must also be identical each run.
		p1, r1 := Distribute(first, 100000, SplitOptions{Reserve: 5000, MaxPer: 40000})
		p2, r2 := Distribute(got, 100000, SplitOptions{Reserve: 5000, MaxPer: 40000})
		if r1 != r2 {
			t.Fatalf("run %d: remainder %d != %d", run, r1, r2)
		}
		for i := range p1 {
			if p1[i].Amount != p2[i].Amount {
				t.Fatalf("run %d: plan %d amount %d != %d", run, i, p1[i].Amount, p2[i].Amount)
			}
		}
	}
}

func TestScoreBreakdownClamped(t *testing.T) {
	// Out-of-range inputs are clamped into [0,1]: negative APR → 0, over-100
	// stability → 1, negative liquidity → 0.
	_, b := Score(Candidate{ExpectedReturnAPR: -10, StabilityScore: 250, LiquidityScore: -5}, Weights{Returns: 1, Stability: 1, Liquidity: 1})
	if b.Returns != 0 {
		t.Errorf("negative APR returns = %g, want 0", b.Returns)
	}
	if b.Stability != 1 {
		t.Errorf("over-100 stability = %g, want 1", b.Stability)
	}
	if b.Liquidity != 0 {
		t.Errorf("negative liquidity = %g, want 0", b.Liquidity)
	}
}

func sumInvariant(t *testing.T, label string, plans []Plan, rem int64, total int64) {
	t.Helper()
	var s int64
	for _, p := range plans {
		s += p.Amount
	}
	if s+rem != total {
		t.Errorf("%s: sum(plans)=%d + remainder=%d = %d, want %d", label, s, rem, s+rem, total)
	}
}

func TestDistributeFillToTarget(t *testing.T) {
	// Helpers: ranked candidates with RemainingToTarget set.
	mkGoal := func(id string, score float64, remaining int64) Ranked {
		return Ranked{
			Candidate: Candidate{ID: id, RemainingToTarget: remaining},
			Score:     score,
		}
	}
	mkFree := func(id string, score float64) Ranked {
		return Ranked{
			Candidate: Candidate{ID: id},
			Score:     score,
		}
	}

	t.Run("empty ranked — invariant holds", func(t *testing.T) {
		plans, rem := DistributeFillToTarget(nil, 1000, SplitOptions{})
		if len(plans) != 0 {
			t.Errorf("want 0 plans, got %d", len(plans))
		}
		sumInvariant(t, "empty", plans, rem, 1000)
	})

	t.Run("ample money — all targets fully funded, leftover spread", func(t *testing.T) {
		ranked := []Ranked{
			mkGoal("g1", 0.8, 200),
			mkGoal("g2", 0.6, 300),
			mkFree("a1", 0.5),
		}
		plans, rem := DistributeFillToTarget(ranked, 10000, SplitOptions{})
		sumInvariant(t, "ample", plans, rem, 10000)
		// Both goals should be fully funded.
		byID := map[string]int64{}
		for _, p := range plans {
			byID[p.Candidate.ID] = p.Amount
		}
		if byID["g1"] < 200 {
			t.Errorf("g1 amount = %d, want >= 200 (fully funded)", byID["g1"])
		}
		if byID["g2"] < 300 {
			t.Errorf("g2 amount = %d, want >= 300 (fully funded)", byID["g2"])
		}
	})

	t.Run("tight money — partial fill of lowest-priority envelope", func(t *testing.T) {
		// Only 250 available after reserve; g1 needs 200, g2 needs 300.
		ranked := []Ranked{
			mkGoal("g1", 0.8, 200),
			mkGoal("g2", 0.6, 300),
		}
		plans, rem := DistributeFillToTarget(ranked, 250, SplitOptions{})
		sumInvariant(t, "tight", plans, rem, 250)
		byID := map[string]int64{}
		for _, p := range plans {
			byID[p.Candidate.ID] = p.Amount
		}
		// g1 is higher priority and its full 200 should be covered.
		if byID["g1"] != 200 {
			t.Errorf("g1 = %d, want 200 (full fill)", byID["g1"])
		}
		// g2 gets the remaining 50.
		if byID["g2"] != 50 {
			t.Errorf("g2 = %d, want 50 (partial fill)", byID["g2"])
		}
	})

	t.Run("zero-target candidates only get leftover spread", func(t *testing.T) {
		ranked := []Ranked{
			mkGoal("g1", 0.9, 100),
			mkFree("a1", 0.7), // no envelope
			mkFree("a2", 0.3),
		}
		plans, rem := DistributeFillToTarget(ranked, 500, SplitOptions{})
		sumInvariant(t, "zero-target", plans, rem, 500)
		byID := map[string]int64{}
		for _, p := range plans {
			byID[p.Candidate.ID] = p.Amount
		}
		// g1 gets at least its 100.
		if byID["g1"] < 100 {
			t.Errorf("g1 = %d, want >= 100", byID["g1"])
		}
		// Free candidates must get something from the spread.
		if byID["a1"] == 0 && byID["a2"] == 0 {
			t.Errorf("free candidates got nothing from spread (a1=%d a2=%d)", byID["a1"], byID["a2"])
		}
	})

	t.Run("reserve respected — reserve > total leaves everything in remainder", func(t *testing.T) {
		ranked := []Ranked{mkGoal("g1", 1, 1000)}
		plans, rem := DistributeFillToTarget(ranked, 100, SplitOptions{Reserve: 500})
		sumInvariant(t, "reserve>total", plans, rem, 100)
		if plans[0].Amount != 0 {
			t.Errorf("plan amount = %d, want 0 (all reserved)", plans[0].Amount)
		}
		if rem != 100 {
			t.Errorf("remainder = %d, want 100", rem)
		}
	})

	t.Run("MaxPer respected in fill pass", func(t *testing.T) {
		// Goal needs 1000 but MaxPer=400; fill pass must cap at 400.
		ranked := []Ranked{mkGoal("g1", 1, 1000)}
		plans, rem := DistributeFillToTarget(ranked, 1500, SplitOptions{MaxPer: 400})
		sumInvariant(t, "maxper-fill", plans, rem, 1500)
		if plans[0].Amount > 400 {
			t.Errorf("g1 amount = %d, exceeds MaxPer=400", plans[0].Amount)
		}
	})

	t.Run("sum invariant holds across varied inputs", func(t *testing.T) {
		cases := []struct {
			label  string
			ranked []Ranked
			total  int64
			opts   SplitOptions
		}{
			{"zero total", []Ranked{mkGoal("g1", 1, 500)}, 0, SplitOptions{}},
			{"negative total", []Ranked{mkGoal("g1", 1, 500)}, -100, SplitOptions{}},
			{"exact fill", []Ranked{mkGoal("g1", 1, 300)}, 300, SplitOptions{}},
			{"reserve + fill", []Ranked{mkGoal("g1", 0.8, 200), mkFree("a1", 0.5)}, 1000, SplitOptions{Reserve: 100}},
			{"multi-goal multi-free", []Ranked{
				mkGoal("g1", 0.9, 150), mkGoal("g2", 0.7, 250), mkFree("a1", 0.6), mkFree("a2", 0.4),
			}, 2000, SplitOptions{Reserve: 200, MaxPer: 800}},
		}
		for _, tc := range cases {
			plans, rem := DistributeFillToTarget(tc.ranked, tc.total, tc.opts)
			sumInvariant(t, tc.label, plans, rem, tc.total)
		}
	})
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
