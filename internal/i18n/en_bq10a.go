// SPDX-License-Identifier: MIT

package i18n

// bq10aKeys adds explicit error and scope copy for the budget dialog fixes.
var bq10aKeys = Catalog{
	"budgets.autoError":              "Auto budget could not finish this calculation: %s",
	"coverAll.currencyError":         "These overages use different currencies. Cover them separately so no amount is compared across currencies.",
	"coverAll.budgetMissing":         "A budget in this cover is no longer available. Close this window and review the current budgets.",
	"coverAll.rolloverBorrowBlocked": "%s carries money between periods, so covering it from next period is unavailable.",
	"sweep.error":                    "The leftover amount cannot be calculated yet: %s",
}

func init() {
	for k, v := range bq10aKeys {
		english[k] = v
	}
}
