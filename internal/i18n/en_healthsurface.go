// SPDX-License-Identifier: MIT

package i18n

// healthSurfaceKeys holds the English strings for the redesigned /health bento
// surface (hero + formula identity, the six in-depth factor tiles, steps,
// history). Merged via init so this file does not touch en.go.
var healthSurfaceKeys = Catalog{
	// Hero
	"health.title":          "Financial health",
	"health.metricsShow":    "Health metrics",
	"health.metricsHide":    "Hide metrics",
	"health.metricsTitle":   "Open the score's variables in the formula builder",
	"health.deficitWarning": "⚠ You're spending more than you earn right now",
	"health.formulaTitle":   "How this number is made — it's a live formula",
	"health.curveSummary":   "How it's measured & scored",
	"health.formulaLabel":   "Composition",
	"health.scoringLabel":   "Scoring",
	"health.exampleLabel":   "Example",
	"health.molNote":        "a live molecule — you can edit it under Formulas",
	"health.derivedNote":    "computed on your device — a live variable you can use in formulas, but not an editable molecule",
	"health.formulaNote":    "Every piece is a variable you can use anywhere, and you can even re-weight your own score by editing health_score under Formulas.",
	"health.formulaHint":    "The score and its six factors are live health_* engine variables — drop any of them into a formula or a dashboard widget.",
	"health.notApplicable":  "Not applicable to you — this factor is left out and the others carry its weight.",
	"health.act":            "Act on this",
	"health.target":         "Target: %s",
	"health.onTarget":       "✓ On target — %s",
	"health.scoreDetail":    "Right now this scores %d out of 100 and counts for %d%% of your overall number.",
	"health.varChipTitle":   "This factor's live engine variable — usable in any formula or dashboard widget",
	"health.stepsTitle":     "Where to focus next",
	"health.privacy":        "Calculated on your device from your own data — never uploaded or shared.",
	"health.historyTitle":   "Score history",
	"health.historyUp":      "Up %d points across your %d monthly readings.",
	"health.historyDown":    "Down %d points across your %d monthly readings.",
	"health.historyFlat":    "Holding steady at %d.",

	// Factor: why it matters + the scoring curve in plain English.
	"health.f.savings.why":       "Keeping a slice of what you earn is the engine of everything else — it funds the emergency cushion, the goals, and the debt payoff.",
	"health.f.savings.curve":     "Scored 0 at a 0% savings rate, rising in a straight line to 100 at 20% or more.",
	"health.f.emergency.why":     "Months of spending covered by liquid cash — the buffer that turns a surprise bill or a lost paycheck into an inconvenience instead of a crisis.",
	"health.f.emergency.curve":   "Scored 0 with nothing saved, 25 at one month, 60 at three, and 100 at six months of cover.",
	"health.f.debt.why":          "The slice of income already spoken for by minimum payments — the higher it is, the less room every other part of the plan has.",
	"health.f.debt.curve":        "Scored 100 with no debt or minimums under 15% of income, 50 at 36%, and 0 past 43%.",
	"health.f.budget.why":        "Whether the plans you set are holding — budgets you stay inside are the difference between intent and outcome.",
	"health.f.budget.curve":      "The score is simply the share of your budgets that are inside their limit.",
	"health.f.utilization.why":   "How much of your total card limit is in use — a key credit-score input, and an early warning of balances building up.",
	"health.f.utilization.curve": "Scored 100 under 10% used, 70 at 30%, sliding to 0 at 80%.",
	"health.f.nw-trend.why":      "The direction of the whole balance sheet over six months — are you actually getting wealthier?",
	"health.f.nw-trend.curve":    "Scored 0 when shrinking 10% or more, about 40 when flat, 80 at +5% growth, and 100 at +10%.",

	// Factor: the value formula (plain prose; the atoms that power it are shown as
	// chips) and a worked example of its impact on the overall score.
	"health.f.savings.formula":     "The share of income you keep — (income − spending) ÷ income — averaged over the last three full months.",
	"health.f.savings.example":     "Keep $1,000 of a $5,000 income (20%) and this maxes at 100. Slip to 5% kept and it falls to 25 — roughly 19 points off your overall score, since it's the heaviest factor (25%).",
	"health.f.emergency.formula":   "Liquid cash divided by your average monthly spending, so the value reads in months of cover.",
	"health.f.emergency.example":   "On $1,500/mo spending, a $4,500 cushion is 3 months → 60. Grow it to $9,000 (6 months) and it maxes at 100 — about +10 points overall.",
	"health.f.debt.formula":        "The sum of your liability minimum payments divided by your monthly income.",
	"health.f.debt.example":        "On $6,000/mo income, $2,000 of minimums is 33% → about 57. Refinance to shave $600 off (down to 23%) and it climbs to ~81 — worth ~5 points overall.",
	"health.f.budget.formula":      "The share of your budgets that are inside their limit this period — the percentage is the score, with no curve.",
	"health.f.budget.example":      "5 of 6 budgets on track scores 83. Bring the one over-budget category back under its limit and it's 100 — a tidy +2 points overall.",
	"health.f.utilization.formula": "Total credit-card balances divided by total credit-card limits.",
	"health.f.utilization.example": "A $6,000 balance against $10,000 of limits is 60% → 28. Pay it down to $3,000 (30%) and it jumps to 70 — about +4 points overall, and a real lift to your credit score.",
	"health.f.nw-trend.formula":    "The six-month change in net worth — today versus six months ago — as a percent.",
	"health.f.nw-trend.example":    "Net worth up 5% over six months scores 80; flat lands near 40; down 10% or more is 0. Steady +10% growth maxes it at 100.",
}

func init() {
	for k, v := range healthSurfaceKeys {
		english[k] = v
	}
}
