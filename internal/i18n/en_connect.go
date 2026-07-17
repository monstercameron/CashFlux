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
}

func init() {
	for k, v := range connectKeys {
		english[k] = v
	}
}
