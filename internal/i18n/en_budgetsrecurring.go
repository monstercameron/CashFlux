// SPDX-License-Identifier: MIT

package i18n

// budgetsRecurringKeys is the copy for the "Recurring in your budgets" tile on
// the /budgets surface (coworker feedback #5: surface the detected recurring
// commitments and — importantly — their frequency, right where budgets are set).
// Own file with an init()-merge so it lands here, not in the user's working en.go.
var budgetsRecurringKeys = Catalog{
	"budgets.recurring.title":         "Recurring in your budgets",
	"budgets.recurring.desc":          "The repeating charges we've detected feeding these budgets, with how often each one hits. Plan around the frequency, not just the amount.",
	"budgets.recurring.totalLabel":    "Committed",
	"budgets.recurring.totalVal":      "≈ %s / month",
	"budgets.recurring.countLabel":    "%d recurring",
	"budgets.recurring.perMonth":      "≈ %s / mo",
	"budgets.recurring.nextDue":       "Next %s",
	"budgets.recurring.uncategorized": "Uncategorized",
	"budgets.recurring.autopay":       "Autopay",
	"budgets.recurring.manage":        "Manage recurring",
}

func init() {
	for k, v := range budgetsRecurringKeys {
		english[k] = v
	}
}
