// SPDX-License-Identifier: MIT

package widgetcatalog

// GroupAssistant collects the assistant_* engine variables — the briefing
// figures the /assistant Insights surface derives (the month's spending story,
// notable category shifts, the top payee) — in the formula picker. Declared
// here beside its metrics so the assistant surface is self-contained.
const GroupAssistant Group = "Assistant"

// assistantMetricMeta labels the fixed assistant variables for the picker, in
// the same order as engineenv.AssistantVarNames.
var assistantMetricMeta = []struct{ Name, Label, Doc string }{
	{"assistant_spend_mtd", "Spent this month", "Total spending so far this month."},
	{"assistant_spend_prev", "Spent last month", "Last month's total spending."},
	{"assistant_spend_pace", "Last month at this point", "What you had spent by this day last month — the like-for-like pace baseline."},
	{"assistant_spend_pace_delta", "Ahead / behind pace", "Spending this month minus last month at the same point. Positive means ahead of last month's pace."},
	{"assistant_highlights", "Category shifts", "How many categories moved materially from their recent monthly norm."},
	{"assistant_top_merchant", "Top merchant spend", "What your biggest payee received over the last 90 days."},
}

// AssistantMetrics exposes the assistant variables (engineenv.addAssistantVars)
// in the formula picker under the Assistant group, so a briefing figure can be
// dropped into a formula or dashboard widget.
func AssistantMetrics() []Metric {
	out := make([]Metric, 0, len(assistantMetricMeta))
	for _, m := range assistantMetricMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupAssistant})
	}
	return out
}
