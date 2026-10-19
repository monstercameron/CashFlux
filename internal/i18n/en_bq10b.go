// SPDX-License-Identifier: MIT

package i18n

var bq10bKeys = Catalog{
	"tasks.budgetOverLimit":           "Budget over limit: %s",
	"tasks.budgetSavingShortfall":     "Savings target needs funding: %s",
	"budgets.compositionNegativeNote": "Refunds and discounts reduce the total; their negative amounts are shown below.",
	"budgets.goalFundedOrdinaryOver":  "Even without goal-funded spending, this budget is %s over its limit.",
}

func init() {
	for key, value := range bq10bKeys {
		english[key] = value
	}
}
