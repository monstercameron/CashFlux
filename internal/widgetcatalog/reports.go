// SPDX-License-Identifier: MIT

package widgetcatalog

// GroupReports collects the report_* engine variables — the figures the
// /reports page derives (period deltas, spending stats, payee concentration,
// burn/runway) — in the formula picker. Declared here (not in the main const
// block) beside its metrics so the reports surface is self-contained.
const GroupReports Group = "Reports"

// reportsMetricMeta labels the fixed report variables for the picker, in the
// same order as engineenv.ReportsVarNames.
var reportsMetricMeta = []struct{ Name, Label, Doc string }{
	{"report_prev_income", "Previous-period income", "Income over the window immediately before the current one."},
	{"report_prev_spend", "Previous-period spending", "Spending over the previous window (positive)."},
	{"report_prev_net", "Previous-period net", "Net cash flow over the previous window."},
	{"report_income_delta_pct", "Income change %", "Income vs the previous window, as a percent."},
	{"report_spend_delta_pct", "Spending change %", "Spending vs the previous window, as a percent (up = spending more)."},
	{"report_avg_expense", "Average purchase", "The average expense transaction this period."},
	{"report_median_expense", "Median purchase", "The middle expense transaction this period (robust to outliers)."},
	{"report_no_spend_days", "No-spend days", "Elapsed days this period with zero spending."},
	{"report_top_payee_spend", "Top payee spending", "What you spent at your single biggest payee this period."},
	{"report_top_payee_pct", "Top payee share %", "That payee's share of all spending this period."},
	{"report_burn", "Monthly burn", "Average monthly spending over your recent full months."},
	{"report_runway_months", "Cash runway (months)", "How long liquid cash lasts at that burn (0 = income covers spending)."},
}

// ReportsMetrics exposes the report variables (engineenv.addReportsVars) in the
// formula picker under the Reports group, so a report figure can be dropped
// into a formula or dashboard widget.
func ReportsMetrics() []Metric {
	out := make([]Metric, 0, len(reportsMetricMeta))
	for _, m := range reportsMetricMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupReports})
	}
	return out
}
