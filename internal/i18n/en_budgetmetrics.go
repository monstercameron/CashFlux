// SPDX-License-Identifier: MIT

package i18n

// budgetMetricsKeys hold the labels for the budget card's quick-metric strip (the
// scannable "time dimension" figures shown on full-width budget cards). Kept out of
// en.go so this addition doesn't touch the concurrent base catalog.
var budgetMetricsKeys = Catalog{
	"budgetMetrics.perDay":   "Left / day",
	"budgetMetrics.daysLeft": "Days left",
	"budgetMetrics.elapsed":  "Elapsed",
}

func init() {
	for k, v := range budgetMetricsKeys {
		english[k] = v
	}
}
