// SPDX-License-Identifier: MIT

package i18n

// bud2Keys contains copy for the BUD2 budgets status strip, member lens, and
// separated Settings and Tools menus.
var bud2Keys = Catalog{
	"bud2.attentionCount":           "%d over or near their limit",
	"bud2.attentionFilter":          "Showing budgets over or near their limit",
	"bud2.statusFits":               "The monthly equivalent plan for %s fits the available income.",
	"bud2.statusNoIncome":           "Your plan for %s is ready to review.",
	"bud2.normalizationUnavailable": "Budgets could not be compared with monthly income.",
	"bud2.monthlyEquivalent":        "Monthly equivalent plan: %s",
	"bud2.normalizedBudgetLine":     "%s is %s; its monthly equivalent is %s.",
	"bud2.periodWeekly":             "weekly",
	"bud2.periodBiweekly":           "every two weeks",
	"bud2.periodSemimonthly":        "twice a month",
	"bud2.periodMonthly":            "monthly",
	"bud2.periodQuarterly":          "quarterly",
	"bud2.periodYearly":             "yearly",
	"bud2.statusOver":               "The monthly equivalent plan for %s is over-assigned.",
	"bud2.statusUnfunded":           "Part of the monthly equivalent plan for %s has not arrived yet.",
	"bud2.resolveAmount":            "Resolve %s",
	"bud2.bringPlanDown":            "Bring plan down",
	"bud2.moreDetails":              "More budget details",
	"bud2.everyone":                 "Everyone",
	"bud2.viewAs":                   "View as",
	"bud2.viewAsMember":             "View as member",
	"bud2.sharedCount":              "Showing %s's share: %d of %d budgets are shared",
	"bud2.settings":                 "Settings",
	"bud2.settingsTitle":            "Budget method, sorting, and list layout",
	"bud2.tools":                    "Tools",
	"bud2.lastMonth":                "See last month's spending",
	"bud2.autoBudget":               "Make a plan from past spending",
	"bud2.monthClose":               "Review the month",
	"bud2.sweep":                    "Move leftover money",
	"bud2.adjustAll":                "Adjust all budget amounts",
}

func init() {
	for key, value := range bud2Keys {
		english[key] = value
	}
}
