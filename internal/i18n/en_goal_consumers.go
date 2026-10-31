// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range (Catalog{
		"budgets.savingsGoalUnavailable":     "Goal progress is unavailable. Check the goal's amounts and currency settings.",
		"dashboard.goalUnavailable":          "Goal progress is unavailable. Check the goal's amounts and currency settings.",
		"dashboard.goalStatesUnavailable":    "%d goals have unavailable progress and are included in Current.",
		"dashboard.goalNonFinancialProgress": "%d of %d completed",
	}) {
		english[key] = value
	}
}
