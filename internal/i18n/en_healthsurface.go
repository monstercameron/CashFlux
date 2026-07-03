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
	"health.curveSummary":   "How it's scored",
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
}

func init() {
	for k, v := range healthSurfaceKeys {
		english[k] = v
	}
}
