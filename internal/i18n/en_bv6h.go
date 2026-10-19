// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"budgets.trackAllCategoryCount":    "Review %d untracked categories",
		"budgets.recurring.unnamedBill":    "Unnamed bill",
		"budgets.recurring.billOne":        "%d bill",
		"budgets.recurring.billMany":       "%d bills",
		"budgets.recurring.excludedTitle":  "Bills not included in totals",
		"budgets.recurring.excludedOne":    "%d bill excluded",
		"budgets.recurring.excludedMany":   "%d bills excluded",
		"budgets.recurring.excludedReason": "These bills have no exchange rate for the budget currency, so their amounts are not included.",
		"budgets.attention.one":            "%d budget needs attention",
		"budgets.attention.many":           "%d budgets need attention",
		"budgets.reviewMonth.oneUnusual":   "%d unusual month excluded",
		"budgets.reviewMonth.manyUnusual":  "%d unusual months excluded",
		"smart.p5.detailNegative":          "Your active goals need about %s/mo. After funding them your typical net is about %s/mo — the goals outpace your surplus, so something has to give.",
	} {
		english[key] = value
	}
}
