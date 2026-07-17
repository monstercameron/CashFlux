// SPDX-License-Identifier: MIT

package i18n

// budgetDensityKeys holds the /budgets list-density toggle copy: full cards vs. the
// compact one-line list for households with many budgets. The button always names the
// layout a click switches TO. Merged via init so this file does not touch en.go.
var budgetDensityKeys = Catalog{
	"budgets.densityCompact": "Compact list",
	"budgets.densityCards":   "Card view",
	"budgets.densityTitle":   "Switch between full budget cards and a compact scannable list",
}

func init() {
	for k, v := range budgetDensityKeys {
		english[k] = v
	}
}
