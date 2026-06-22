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
	// GoalProgress is how complete a linked savings goal is, 0..1. It powers the
	// goal-progress criterion: destinations funding goals nearest completion
	// score highest, favouring finishing what's almost done over starting anew.
	// Non-goal destinations leave it 0 and so score 0 on that criterion.
	GoalProgress float64
	// RemainingToTarget is the minor-unit amount still needed to reach the
	// candidate's target (0 means no target / unbounded).
	RemainingToTarget int64
}

// Weights expresses how much a user cares about each criterion. They need not
// sum to 1; scoring normalizes by their total.
type Weights struct {
	Returns       float64
	Stability     float64
	Liquidity     float64
	DebtReduction float64
	GoalProgress  float64
}

// Breakdown holds each criterion's normalized score (0..1) for a candidate, so
// the result is explainable.
type Breakdown struct {
	Returns       float64
	Stability     float64
	Liquidity     float64
	DebtReduction float64
	GoalProgress  float64
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
		Returns:      clamp01(c.ExpectedReturnAPR / returnsCap),
		Stability:    clamp01(float64(c.StabilityScore) / 100),
		Liquidity:    clamp01(float64(c.LiquidityScore) / 100),
		GoalProgress: clamp01(c.GoalProgress),
	}
	if c.DebtReduction {
		b.DebtReduction = 1
	}

	total := w.Returns + w.Stability + w.Liquidity + w.DebtReduction + w.GoalProgress
	if total <= 0 {
		total = 1
	}
	score := (w.Returns*b.Returns + w.Stability*b.Stability + w.Liquidity*b.Liquidity +
		w.DebtReduction*b.DebtReduction + w.GoalProgress*b.GoalProgress) / total
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

// Plan is one destination's allocated amount, in minor currency units.
type Plan struct {
	Candidate Candidate
	Amount    int64
}

// SplitOptions configures how Distribute spreads money across destinations.
type SplitOptions struct {
	Reserve int64 // emergency buffer held back from the total before splitting
	MaxPer  int64 // cap per destination (0 = no cap)
}

// DistributeFillToTarget allocates total to ranked candidates using a
// fill-then-spread strategy:
//  1. Reserve opts.Reserve from total; work with the remainder.
//  2. In ranked order, give each candidate up to min(RemainingToTarget,
//     available, opts.MaxPer) — candidates with RemainingToTarget==0 are
//     skipped in this fill pass (they have no envelope to fill).
//  3. Spread any money still remaining across ALL candidates via the
//     existing score-weighted Distribute, respecting opts.MaxPer minus
//     what each candidate already received in the fill pass.
//  4. Return one Plan per input candidate (ranked order) and the final
//     unallocated remainder.
//
// INVARIANT: sum(plan.Amount) + remainder == total for every input.
func DistributeFillToTarget(ranked []Ranked, total int64, opts SplitOptions) ([]Plan, int64) {
	plans := make([]Plan, len(ranked))
	for i, r := range ranked {
		plans[i] = Plan{Candidate: r.Candidate}
	}
	if total <= 0 || len(ranked) == 0 {
		return plans, total
	}

	available := total - opts.Reserve
	if available < 0 {
		available = 0
	}

	// Fill pass: in ranked order give each goal-envelope candidate up to its
	// remaining target, capped by available and by MaxPer when set.
	fillAmount := make(map[string]int64, len(ranked))
	for _, r := range ranked {
		if available <= 0 {
			break
		}
		if r.Candidate.RemainingToTarget <= 0 {
			continue
		}
		give := r.Candidate.RemainingToTarget
		if opts.MaxPer > 0 && give > opts.MaxPer {
			give = opts.MaxPer
		}
		if give > available {
			give = available
		}
		fillAmount[r.Candidate.ID] = give
		available -= give
	}

	// Spread pass: distribute whatever is left across all candidates via
	// Distribute, honouring remaining headroom under MaxPer for each candidate.
	var spreadPlans []Plan
	if available > 0 {
		adjusted := make([]Ranked, 0, len(ranked))
		adjustedMaxPer := make(map[string]int64, len(ranked))
		for _, r := range ranked {
			if opts.MaxPer > 0 {
				remaining := opts.MaxPer - fillAmount[r.Candidate.ID]
				if remaining <= 0 {
					// Already at or above cap — exclude from spread.
					continue
				}
				adjustedMaxPer[r.Candidate.ID] = remaining
			}
			adjusted = append(adjusted, r)
		}
		if len(adjusted) > 0 {
			// Build per-candidate adjusted opts. Because Distribute takes a single
			// MaxPer we apply the lowest adjusted cap across the slice, which may
			// over-constrain some candidates. Instead, run Distribute with no cap
			// and then clamp each spread amount manually to the candidate's headroom.
			spreadRaw, _ := Distribute(adjusted, available, SplitOptions{})
			spreadPlans = make([]Plan, len(spreadRaw))
			var totalSpread int64
			for i, p := range spreadRaw {
				amt := p.Amount
				if opts.MaxPer > 0 {
					cap := adjustedMaxPer[p.Candidate.ID]
					if amt > cap {
						amt = cap
					}
				}
				spreadPlans[i] = Plan{Candidate: p.Candidate, Amount: amt}
				totalSpread += amt
			}
			_ = totalSpread
		}
	}

	// Build spread lookup by ID.
	spreadByID := make(map[string]int64, len(spreadPlans))
	for _, p := range spreadPlans {
		spreadByID[p.Candidate.ID] = p.Amount
	}

	// Merge fill + spread into the output plans.
	for i, r := range ranked {
		plans[i].Amount = fillAmount[r.Candidate.ID] + spreadByID[r.Candidate.ID]
	}

	// Compute remainder from the invariant so it holds exactly.
	var sumAmounts int64
	for _, p := range plans {
		sumAmounts += p.Amount
	}
	return plans, total - sumAmounts
}

// Distribute splits total (minor units) across ranked candidates in proportion to
// their scores, after holding back Reserve and capping each destination at MaxPer.
// It returns one Plan per candidate (in ranked order) plus the unallocated
// remainder — the reserve, anything left by per-destination caps, and integer
// rounding. Amounts are whole minor units and never exceed the available total.
// When every score is zero (or absent) the available amount is split evenly.
func Distribute(ranked []Ranked, total int64, opts SplitOptions) ([]Plan, int64) {
	plans := make([]Plan, len(ranked))
	for i, r := range ranked {
		plans[i] = Plan{Candidate: r.Candidate}
	}
	available := total - opts.Reserve
	if available < 0 {
		available = 0
	}
	if available == 0 || len(ranked) == 0 {
		return plans, total
	}

	var sumScore float64
	for _, r := range ranked {
		if r.Score > 0 {
			sumScore += r.Score
		}
	}

	var allocated int64
	for i, r := range ranked {
		var amt int64
		if sumScore > 0 {
			score := r.Score
			if score < 0 {
				score = 0
			}
			amt = int64(float64(available) * (score / sumScore))
		} else {
			amt = available / int64(len(ranked))
		}
		if opts.MaxPer > 0 && amt > opts.MaxPer {
			amt = opts.MaxPer
		}
		plans[i].Amount = amt
		allocated += amt
	}
	return plans, total - allocated
}
