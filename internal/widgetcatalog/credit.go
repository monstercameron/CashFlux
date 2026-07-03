// SPDX-License-Identifier: MIT

package widgetcatalog

// GroupCredit collects the credit_* factor atoms — the credit-health proxy's
// per-factor scores, exact weights, and pay-down targets — in the formula
// picker. The headline credit_proxy itself is a MOLECULE (a formula over these
// atoms, in DefaultMolecules), so it appears with its derivation among the
// compound figures; this group holds the addressable pieces it is built from.
const GroupCredit Group = "Credit health"

// creditMetricMeta labels the fixed credit atoms for the picker, in the same
// order as engineenv.CreditVarNames.
var creditMetricMeta = []struct{ Name, Label, Doc string }{
	{"credit_util_score", "Utilization factor", "0–100 score for aggregate card utilization (100 at 10% or less)."},
	{"credit_util_weight", "Utilization weight", "The utilization factor's exact share of the credit proxy."},
	{"credit_ontime_score", "On-time factor", "0–100 on-time payment proxy from your card due days."},
	{"credit_ontime_weight", "On-time weight", "The on-time factor's exact share of the credit proxy."},
	{"credit_age_score", "Account-age factor", "0–100 proxy for how long your cards have been open."},
	{"credit_age_weight", "Age weight", "The age factor's exact share of the credit proxy."},
	{"credit_pay_to_30", "Pay to reach 30%", "Total to pay so every card sits at or under 30% utilization."},
	{"credit_pay_to_10", "Pay to reach 10%", "Total to pay so every card sits at or under 10% utilization."},
}

// CreditMetrics exposes the credit atoms (engineenv.addCreditVars) in the
// formula picker under the Credit health group, so any piece of the proxy can
// be dropped into a formula or dashboard widget.
func CreditMetrics() []Metric {
	out := make([]Metric, 0, len(creditMetricMeta))
	for _, m := range creditMetricMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupCredit})
	}
	return out
}
