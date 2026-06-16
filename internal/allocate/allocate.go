// Package allocate ranks candidate destinations for new capital (savings,
// investments, high-interest debt) by scoring each on returns, stability,
// liquidity, and debt reduction, then combining those by a user's weight
// profile. Scoring is deterministic and fully explainable — every ranked result
// carries its per-criterion breakdown so the UI can show why, not a black box.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package allocate

import "sort"

// returnsCap is the expected-return APR (percent) treated as a perfect returns
// score; anything at or above it normalizes to 1.0.
const returnsCap = 15.0

// Candidate is a place capital could go.
type Candidate struct {
	ID                string
	Name              string
	ExpectedReturnAPR float64 // annual percent (for debt, its interest rate as a guaranteed return)
	StabilityScore    int     // 0..100, how stable/safe the destination is
	LiquidityScore    int     // 0..100, how easily funds can be withdrawn
	DebtReduction     bool    // true when this is paying down a liability (guaranteed return)
}

// Weights expresses how much a user cares about each criterion. They need not
// sum to 1; scoring normalizes by their total.
type Weights struct {
	Returns       float64
	Stability     float64
	Liquidity     float64
	DebtReduction float64
}

// Breakdown holds each criterion's normalized score (0..1) for a candidate, so
// the result is explainable.
type Breakdown struct {
	Returns       float64
	Stability     float64
	Liquidity     float64
	DebtReduction float64
}

// Ranked is a candidate with its overall score (0..1) and breakdown.
type Ranked struct {
	Candidate Candidate
	Score     float64
	Breakdown Breakdown
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// Score returns a candidate's overall score (0..1, the weight-normalized average
// of its criteria) and the per-criterion breakdown.
func Score(c Candidate, w Weights) (float64, Breakdown) {
	b := Breakdown{
		Returns:   clamp01(c.ExpectedReturnAPR / returnsCap),
		Stability: clamp01(float64(c.StabilityScore) / 100),
		Liquidity: clamp01(float64(c.LiquidityScore) / 100),
	}
	if c.DebtReduction {
		b.DebtReduction = 1
	}

	total := w.Returns + w.Stability + w.Liquidity + w.DebtReduction
	if total <= 0 {
		total = 1
	}
	score := (w.Returns*b.Returns + w.Stability*b.Stability + w.Liquidity*b.Liquidity + w.DebtReduction*b.DebtReduction) / total
	return score, b
}

// Rank scores every candidate and returns them sorted by score (highest first).
// Ties keep input order (stable sort).
func Rank(candidates []Candidate, w Weights) []Ranked {
	out := make([]Ranked, 0, len(candidates))
	for _, c := range candidates {
		s, b := Score(c, w)
		out = append(out, Ranked{Candidate: c, Score: s, Breakdown: b})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// Constraints narrows which candidates are eligible before ranking. It is a
// struct (not a bare set) so future constraints — caps, required destinations —
// can be added without changing call sites.
type Constraints struct {
	Exclude map[string]bool // candidate IDs to leave out of the ranking entirely
}

// Eligible reports whether a candidate passes the constraints.
func (c Constraints) Eligible(cand Candidate) bool {
	return !c.Exclude[cand.ID]
}

// RankWith filters the candidates by the constraints, then ranks the survivors.
// With zero-value constraints it is identical to Rank.
func RankWith(candidates []Candidate, w Weights, cons Constraints) []Ranked {
	filtered := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if cons.Eligible(c) {
			filtered = append(filtered, c)
		}
	}
	return Rank(filtered, w)
}
