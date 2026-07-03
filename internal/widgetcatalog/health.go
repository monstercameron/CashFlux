// SPDX-License-Identifier: MIT

package widgetcatalog

// GroupHealth collects the health_* factor atoms — the financial-health model's
// per-factor scores, exact weights, penalty, and raw values — in the formula
// picker. The headline health_score itself is a MOLECULE (a formula over these
// atoms, in DefaultMolecules), so it appears with its derivation among the
// compound figures; this group holds the addressable pieces it is built from.
const GroupHealth Group = "Health factors"

// healthMetricMeta labels the fixed health atoms for the picker, in the same
// order as engineenv.HealthVarNames.
var healthMetricMeta = []struct{ Name, Label, Doc string }{
	{"health_savings", "Savings factor", "0–100 score for your trailing savings rate (100 at 20%+)."},
	{"health_savings_weight", "Savings weight", "The savings factor's exact share of the overall score."},
	{"health_emergency", "Emergency-fund factor", "0–100 score for months of spending covered by liquid cash (100 at 6 months)."},
	{"health_emergency_weight", "Emergency weight", "The emergency-fund factor's exact share of the overall score."},
	{"health_debt", "Debt-burden factor", "0–100 score for minimum debt payments vs income (100 under 15%)."},
	{"health_debt_weight", "Debt weight", "The debt factor's exact share of the overall score."},
	{"health_budget", "Budget factor", "0–100 score: the share of budgets inside their limit."},
	{"health_budget_weight", "Budget weight", "The budget factor's exact share of the overall score."},
	{"health_utilization", "Utilization factor", "0–100 score for aggregate credit-card utilization (100 under 10%)."},
	{"health_utilization_weight", "Utilization weight", "The utilization factor's exact share of the overall score."},
	{"health_trend", "Net-worth-trend factor", "0–100 score for the 6-month net-worth trajectory (100 at +10%)."},
	{"health_trend_weight", "Trend weight", "The trend factor's exact share of the overall score."},
	{"health_penalty", "Deficit penalty", "Flat deduction applied when spending exceeds income (else 0)."},
	{"health_emergency_months", "Emergency months", "Liquid cash divided by average monthly spending."},
	{"health_obligation_pct", "Obligation ratio %", "Minimum debt payments as a percent of monthly income."},
}

// HealthMetrics exposes the health atoms (engineenv.addHealthVars) in the
// formula picker under the Health factors group, so any piece of the score can
// be dropped into a formula or dashboard widget.
func HealthMetrics() []Metric {
	out := make([]Metric, 0, len(healthMetricMeta))
	for _, m := range healthMetricMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupHealth})
	}
	return out
}
