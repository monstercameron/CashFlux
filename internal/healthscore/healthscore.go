// SPDX-License-Identifier: MIT

// Package healthscore computes a deterministic, explainable 0–100 financial-health
// score for a household from a handful of already-derived signals (savings rate,
// emergency-fund months, debt-payment burden, budget adherence, credit utilization).
//
// It is pure (no syscall/js, no I/O) so it unit-tests on native Go and runs the same
// in the wasm build. The UI layer assembles Inputs from the ledger/reports/budgeting
// packages; this package owns only the model: factor scoring, weighting with
// proportional re-normalization when a factor is inapplicable, banding, and the
// plain-English next steps. Everything here is reproducible and explainable — no AI,
// no black box (see SPEC §determinism & explainability).
package healthscore

import (
	"fmt"
	"math"
	"sort"
)

// Band is the qualitative tier a score falls into. NoData is returned when fewer
// than two factors are applicable, so the UI can show "not enough yet" instead of a
// misleading number.
type Band string

const (
	BandExcellent Band = "Excellent"
	BandGood      Band = "Good"
	BandFair      Band = "Fair"
	BandNeedsWork Band = "Needs work"
	BandCritical  Band = "Critical"
	BandNoData    Band = "Not enough data"
)

// minApplicable is the fewest factors that must be present to produce a real score;
// below this the result is BandNoData (one factor isn't a "health" picture).
const minApplicable = 2

// Inputs are the pre-derived signals, one per factor, each paired with a flag for
// whether that factor applies to this household. A factor that doesn't apply (e.g.
// no credit cards) is dropped and its weight redistributed — it is NOT scored zero.
type Inputs struct {
	// SavingsRatePct is the trailing-average savings rate (income kept), as a whole
	// percent; may be negative when spending exceeds income. Applies iff HasIncome.
	SavingsRatePct int
	HasIncome      bool

	// EmergencyMonths is liquid cash ÷ average monthly spending. Applies iff HasLiquidData.
	EmergencyMonths float64
	HasLiquidData   bool

	// ObligationRatioPct is the sum of liability minimum payments ÷ monthly income,
	// as a whole percent. Honestly NOT a full DTI (only minimums are modelled).
	// Applies iff HasIncome; when there are no liabilities it scores 100 (zero debt
	// is good — it is applicable, not dropped).
	ObligationRatioPct int
	HasLiabilities     bool

	// BudgetAdherencePct is the share of budgets within their limit. Applies iff HasBudgets.
	BudgetAdherencePct int
	HasBudgets         bool

	// AggUtilizationPct is aggregate revolving utilization (total card balance ÷ total
	// limit), as a whole percent. Applies iff HasCredit.
	AggUtilizationPct int
	HasCredit         bool
}

// Factor is one scored dimension, with everything the UI needs to explain it: the
// current value, its 0–100 score, its share of the overall score after
// re-normalization, and the target the user is aiming for.
type Factor struct {
	Key             string
	Label           string
	Value           string // human display of the current value, e.g. "12%", "2.3 mo"
	Score           int    // 0–100
	ContributionPct int    // this factor's weight share of the overall score (post re-normalize)
	Target          string // plain-English goal, e.g. "20% or more"
	Applicable      bool
}

// Step is one prioritized, plain-English action drawn from the weakest factors.
// TimeFraming is optional context the UI/builder may fill (e.g. "~8 months away");
// the model leaves it empty when it can't be derived from the score alone.
type Step struct {
	Factor      string
	Action      string
	Target      string
	TimeFraming string
}

// Result is the full, explainable output: the overall score + band, every factor
// (applicable or not), the prioritized steps, and a NegativeCashFlow flag the UI
// surfaces as a warning WITHOUT distorting the aggregate math.
type Result struct {
	Score            int
	Band             Band
	Factors          []Factor
	Steps            []Step
	NegativeCashFlow bool
	ApplicableCount  int
}

type factorDef struct {
	key, label, target string
	weight             float64
	applicable         bool
	rawScore           int
	value              string
}

// Evaluate runs the deterministic model. The overall score is the weighted average
// of applicable factor scores, with inapplicable factors dropped and their weight
// redistributed proportionally across the rest (so the weights of applicable factors
// always sum to 1). Fewer than two applicable factors yields BandNoData.
func Evaluate(in Inputs) Result {
	defs := []factorDef{
		{
			key: "savings", label: "Savings rate", target: "20% or more", weight: 0.25,
			applicable: in.HasIncome, rawScore: savingsScore(in.SavingsRatePct),
			value: fmt.Sprintf("%d%%", in.SavingsRatePct),
		},
		{
			key: "emergency", label: "Emergency fund", target: "3–6 months", weight: 0.25,
			applicable: in.HasLiquidData, rawScore: emergencyScore(in.EmergencyMonths),
			value: fmt.Sprintf("%.1f mo", in.EmergencyMonths),
		},
		{
			key: "debt", label: "Debt payments", target: "under 36% of income", weight: 0.20,
			applicable: in.HasIncome, rawScore: obligationScore(in.ObligationRatioPct, in.HasLiabilities),
			value: obligationValue(in.ObligationRatioPct, in.HasLiabilities),
		},
		{
			key: "budget", label: "Budget adherence", target: "100% on track", weight: 0.15,
			applicable: in.HasBudgets, rawScore: clampPct(in.BudgetAdherencePct),
			value: fmt.Sprintf("%d%%", clampPct(in.BudgetAdherencePct)),
		},
		{
			key: "utilization", label: "Credit utilization", target: "under 30%", weight: 0.15,
			applicable: in.HasCredit, rawScore: utilizationScore(in.AggUtilizationPct),
			value: fmt.Sprintf("%d%%", clampPct(in.AggUtilizationPct)),
		},
	}

	// Sum of weights of applicable factors (the denominator for re-normalization).
	var applWeight float64
	applCount := 0
	for _, d := range defs {
		if d.applicable {
			applWeight += d.weight
			applCount++
		}
	}

	res := Result{
		NegativeCashFlow: in.HasIncome && in.SavingsRatePct < 0,
		ApplicableCount:  applCount,
		Factors:          make([]Factor, 0, len(defs)),
	}

	for _, d := range defs {
		f := Factor{Key: d.key, Label: d.label, Value: d.value, Target: d.target, Applicable: d.applicable}
		if d.applicable {
			f.Score = d.rawScore
			if applWeight > 0 {
				f.ContributionPct = int(math.Round(d.weight / applWeight * 100))
			}
		} else {
			f.Value = "—"
		}
		res.Factors = append(res.Factors, f)
	}

	if applCount < minApplicable || applWeight <= 0 {
		res.Band = BandNoData
		return res
	}

	var weighted float64
	for _, d := range defs {
		if d.applicable {
			weighted += float64(d.rawScore) * (d.weight / applWeight)
		}
	}
	res.Score = clampPct(int(math.Round(weighted)))
	res.Band = bandFor(res.Score)
	res.Steps = buildSteps(defs)
	return res
}

// bandFor maps a 0–100 score to its tier (five tiers; "Critical" distinguishes a
// dire score from a merely weak one so the coaching copy can match the urgency).
func bandFor(score int) Band {
	switch {
	case score >= 80:
		return BandExcellent
	case score >= 60:
		return BandGood
	case score >= 40:
		return BandFair
	case score >= 25:
		return BandNeedsWork
	default:
		return BandCritical
	}
}

// buildSteps returns up to three actions drawn from the lowest-scoring applicable
// factors (a perfect factor needs no step). Stable order: lowest score first.
func buildSteps(defs []factorDef) []Step {
	type cand struct {
		def factorDef
	}
	var cands []cand
	for _, d := range defs {
		if d.applicable && d.rawScore < 90 {
			cands = append(cands, cand{d})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].def.rawScore < cands[j].def.rawScore })
	if len(cands) > 3 {
		cands = cands[:3]
	}
	steps := make([]Step, 0, len(cands))
	for _, c := range cands {
		steps = append(steps, Step{Factor: c.def.label, Action: stepAction(c.def.key), Target: c.def.target})
	}
	return steps
}

func stepAction(key string) string {
	switch key {
	case "savings":
		return "Trim spending or add income so you keep more of what you earn"
	case "emergency":
		return "Set aside cash until you have a few months of expenses saved"
	case "debt":
		return "Pay down high-payment debts to lower your monthly obligations"
	case "budget":
		return "Bring over-budget categories back under their limits"
	case "utilization":
		return "Pay down card balances to lower your credit utilization"
	default:
		return "Review this area"
	}
}

// ---- factor scoring (each clamps to 0–100) ----

// savingsScore: 0% and below → 0; 20%+ → 100; linear between.
func savingsScore(pct int) int {
	if pct <= 0 {
		return 0
	}
	if pct >= 20 {
		return 100
	}
	return clampPct(int(math.Round(float64(pct) * 5))) // 20% → 100
}

// emergencyScore: piecewise-linear through (0,0)(1,25)(3,60)(6,100).
func emergencyScore(months float64) int {
	switch {
	case months <= 0:
		return 0
	case months < 1:
		return clampPct(int(math.Round(months * 25)))
	case months < 3:
		return clampPct(int(math.Round(25 + (months-1)*17.5)))
	case months < 6:
		return clampPct(int(math.Round(60 + (months-3)*(40.0/3.0))))
	default:
		return 100
	}
}

// obligationScore: no liabilities → 100 (zero debt is good); else piecewise-linear
// through (15,100)(36,50)(43,0).
func obligationScore(pct int, hasLiabilities bool) int {
	if !hasLiabilities {
		return 100
	}
	p := float64(pct)
	switch {
	case p <= 15:
		return 100
	case p < 36:
		return clampPct(int(math.Round(100 - (p-15)*(50.0/21.0))))
	case p < 43:
		return clampPct(int(math.Round(50 - (p-36)*(50.0/7.0))))
	default:
		return 0
	}
}

func obligationValue(pct int, hasLiabilities bool) string {
	if !hasLiabilities {
		return "no debt"
	}
	return fmt.Sprintf("%d%%", pct)
}

// utilizationScore: piecewise-linear through (10,100)(30,70)(80,0).
func utilizationScore(pct int) int {
	p := float64(clampPct(pct))
	switch {
	case p <= 10:
		return 100
	case p < 30:
		return clampPct(int(math.Round(100 - (p-10)*1.5)))
	case p < 80:
		return clampPct(int(math.Round(70 - (p-30)*1.4)))
	default:
		return 0
	}
}

func clampPct(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
