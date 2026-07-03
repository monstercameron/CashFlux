// SPDX-License-Identifier: MIT

package widgetcatalog

// GroupNetWorth collects the networth_* engine variables — the balance-sheet
// figures the /networth page derives (month-to-date change, the asset-class
// composition, the liquid share) — in the formula picker. Declared here beside
// its metrics so the net-worth surface is self-contained.
const GroupNetWorth Group = "Net worth"

// netWorthMetricMeta labels the fixed net-worth variables for the picker, in
// the same order as engineenv.NetWorthVarNames.
var netWorthMetricMeta = []struct{ Name, Label, Doc string }{
	{"networth_change", "Change this month", "How much net worth moved since the month started."},
	{"networth_change_pct", "Change this month %", "That change as a percent of the month-start figure."},
	{"networth_cash", "Cash assets", "Checking, debit, savings, and cash account balances combined."},
	{"networth_invested", "Invested assets", "Investment, retirement, and crypto account balances combined."},
	{"networth_property", "Property & vehicles", "Property and vehicle values combined."},
	{"networth_other_assets", "Other assets", "Every other asset account combined."},
	{"networth_liquid_pct", "Liquid share %", "Cash-type assets as a percent of everything you own."},
}

// NetWorthMetrics exposes the net-worth variables (engineenv.addNetWorthVars)
// in the formula picker under the Net worth group, so a balance-sheet figure
// can be dropped into a formula or dashboard widget.
func NetWorthMetrics() []Metric {
	out := make([]Metric, 0, len(netWorthMetricMeta))
	for _, m := range netWorthMetricMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupNetWorth})
	}
	return out
}
