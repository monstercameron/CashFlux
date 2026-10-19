// SPDX-License-Identifier: MIT

package i18n

// BR2ED corrects the budget row actions and gives the inline limit editor
// names that include the budget being changed.
var br2edKeys = Catalog{
	"budgets.actionCoverFromOtherMenu": "Cover from another budget",
	"budgets.actionEditFormulasPlain":  "Calculate the limit with a formula",
	"budgets.pauseBudgetForMonth":      "Pause this budget for a month",
	"budgets.limitTitleSuffix":         "limit",
	"budgets.baseLimitNameSuffix":      "base limit",
}

func init() {
	for key, value := range br2edKeys {
		english[key] = value
	}
}
