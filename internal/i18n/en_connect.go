// SPDX-License-Identifier: MIT

package i18n

// connectKeys holds the English strings for the cross-page "connective layer" —
// the features that make the pages reason together (budget drivers, item-level
// deep-links, bill→budget fit) and celebrate wins (milestones). Kept in its own
// file (like en_smart.go / en_home.go) so these additions never touch the churned
// en.go, and registered into the shared catalog at init.
var connectKeys = Catalog{
	// Budget "what's driving this?" driver panel.
	"budgets.driversShow":              "What's driving this",
	"budgets.driversHide":              "Hide drivers",
	"budgets.driversNone":              "No matching charges this period yet.",
	"budgets.driverRecurring":          "recurring",
	"budgets.driverDrillAria":          "See all %s charges",
	"budgets.driverDrillRecurringAria": "Manage the recurring charge from %s",

	// Bill → budget-fit chip: does this upcoming charge still fit the budget that
	// tracks its category, for the period it's due?
	"bills.budgetFits":    "Fits %s · %s left",
	"bills.budgetOver":    "%s over %s",
	"bills.budgetFitAria": "Open the %s budget",

	// Milestone celebrations: the positive counterweight to the warning feed.
	"wins.title":         "Recent wins",
	"wins.subtitle":      "A quick note on what's going right.",
	"wins.newBadge":      "New",
	"wins.goalTitle":     "Goal reached: %s",
	"wins.goalMsg":       "You fully funded %s. Time to pick the next one.",
	"wins.netWorthTitle": "Net worth past %s",
	"wins.netWorthMsg":   "Your net worth crossed %s — the slow climb is working.",
	"wins.noSpendTitle":  "%d days, no spending",
	"wins.noSpendMsg":    "You've gone %d days without a purchase. Nice restraint.",
	"wins.keptTitle":     "%d budgets kept last month",
	"wins.keptMsg":       "You finished last month within %d of your budgets. Keep it up.",
}

func init() {
	for k, v := range connectKeys {
		english[k] = v
	}
}
