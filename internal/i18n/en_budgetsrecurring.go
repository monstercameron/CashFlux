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

	// Feedback #6: the future-period projection tile (shows only when the period
	// control is paged into the future, so a future month is never empty).
	"budgets.future.title": "Projected for %s",
	"budgets.future.desc":  "You're viewing a future period. Here's what your recurring bills and income are set to do — projected, not yet real.",
	"budgets.future.in":    "Coming in",
	"budgets.future.out":   "Going out",
	"budgets.future.net":   "Net",
	"budgets.future.none":  "No recurring activity lands in this period.",
	"budgets.future.more":  "+ %d more",
}

func init() {
	for k, v := range budgetsRecurringKeys {
		english[k] = v
	}
}
