// SPDX-License-Identifier: MIT

package i18n

// br2recKeys corrects the recurring budget panel's scope and makes the year
// plan figure explicitly identify the calendar year it describes.
var br2recKeys = Catalog{
	"budgets.recurring.allCategoriesTitle": "Recurring bills (all categories)",
	"budgets.recurring.coverageDesc":       "Active recurring expenses, grouped by whether a budget covers their category.",
	"budgets.recurring.monthlyBillsLabel":  "Recurring bills",
	"budgets.recurring.billCount":          "%d bills",
	"budgets.recurring.coveredTitle":       "Covered by a budget",
	"budgets.recurring.unbudgetedTitle":    "Not budgeted",
	"budgets.recurring.unbudgetedMonthly":  "(%s/mo)",
	"budgets.recurring.billFoldOne":        "%[2]s a month · %[1]d bill",
	"budgets.recurring.billFoldMany":       "%[2]s a month · %[1]d bills",
	"budgets.planYearStableHint":           "%d plan · %s",
}

func init() {
	for key, value := range br2recKeys {
		english[key] = value
	}
}
