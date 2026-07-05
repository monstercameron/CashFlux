// SPDX-License-Identifier: MIT

package i18n

// budgetPolishKeys holds v1.0 copy fixes for the Budgets screen: a singular
// variant of the over-budget banner (the plural read "1 budgets are over"), and
// a confirm prompt for the bulk 50/30/20 template create. Merged via init so
// this file does not touch en.go.
var budgetPolishKeys = Catalog{
	"budgets.overBannerOne":    "1 budget is over by %s total — review and cover the overspend.",
	"budgets.tmplConfirm":      "Create %s from the 50/30/20 template? You can edit or delete them afterward.",
	"budgets.tmplConfirmBtn":   "Create budgets",
	"budgets.tmplNothingToAdd": "Every 50/30/20 category already has a budget — nothing to add.",
}

func init() {
	for k, v := range budgetPolishKeys {
		english[k] = v
	}
}
