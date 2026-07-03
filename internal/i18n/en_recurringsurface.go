// SPDX-License-Identifier: MIT

package i18n

// recurringSurfaceKeys holds the English strings for the redesigned /recurring
// Scheduled surface (bento tiles + the add/edit flip modal). Merged via init so
// this file does not touch en.go; mirrors the en_recurring_tabs.go pattern.
var recurringSurfaceKeys = Catalog{
	// Hero
	"recurring.heroLabel":    "Monthly recurring net",
	"recurring.figIn":        "Money in / mo",
	"recurring.figOut":       "Money out / mo",
	"recurring.figFlows":     "Active flows",
	"recurring.figOverdue":   "Overdue",
	"recurring.figNextDue":   "Next due",
	"recurring.overviewHint": "Bills, paychecks, and subscriptions that repeat — and what they add up to each month.",

	// Toolbar
	"recurring.addFlow":      "Add recurring",
	"recurring.addFlowTitle": "Track a repeating bill, paycheck, or subscription",

	// Upcoming
	"recurring.upcomingTitle": "Next 30 days",
	"recurring.upcomingHint":  "Every due date in the next month, from your scheduled flows.",
	"recurring.upcomingNone":  "Nothing due in the next 30 days.",
	"recurring.upcomingMeta":  "%s due · %s out · %s in",
	"recurring.overdue":       "Overdue",
	"recurring.upcomingMore":  "+ %d more in this window",

	// Flows list ("recurring flows" is the one vocabulary — the Scheduled tab holds
	// the page's recurring flows; no competing "scheduled flows" phrasing).
	"recurring.flowsTitle":    "All recurring flows",
	"recurring.flowsHint":     "Each flow shows what it adds up to per month, whatever its cadence.",
	"recurring.perMonth":      "%s / mo",
	"recurring.shareOfOut":    "Share of monthly outflow",
	"recurring.shareLabel":    "%.0f%% of outflow",
	"recurring.viewAccount":   "View account",
	"recurring.viewTxns":      "View transactions",
	"recurring.viewTxnsTitle": "See this flow's transactions, pre-filtered to its account and category",
	"recurring.viewBudget":    "View budget",
	"recurring.varHint":       "This flow's engine variable — use it in any formula or dashboard widget.",
	"recurring.metricsShow":   "Schedule metrics",
	"recurring.metricsHide":   "Hide metrics",
	"recurring.metricsTitle":  "Recurring metrics",
	"recurring.formulaHint":   "Build a figure from the recurring_* variables (money in/out per month, each flow's monthly equivalent) and any other engine value.",
	"recurring.moreActions":   "More actions",
	"recurring.deleteConfirm": "Delete “%s”? Its schedule disappears from forecasts and the bills list.",
	"recurring.autopostBadge": "Auto-post",
	"recurring.autopostHint":  "Due occurrences post into the ledger automatically.",

	// Add/edit modal
	"recurring.newTitle":             "Add a recurring flow",
	"recurring.editTitleModal":       "Edit recurring flow",
	"recurring.directionLabel":       "Direction",
	"recurring.dirOut":               "Money out",
	"recurring.dirIn":                "Money in",
	"recurring.amountLabel":          "Amount per occurrence (%s)",
	"recurring.accountOptional":      "Account (optional)",
	"recurring.categoryOptional":     "Category (optional)",
	"recurring.autopostNeedsAccount": "Pick an account above to enable auto-posting.",
	"recurring.saveFlow":             "Save flow",
	"recurring.addedFlash":           "Added “%s” ✓",

	// Detected
	"recurring.detectedSection": "Found in your history",
	"recurring.done":            "Done",
	"subs.detectPrefsTitle":     "Detection preferences",
}

func init() {
	for k, v := range recurringSurfaceKeys {
		english[k] = v
	}
}
