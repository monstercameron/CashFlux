// SPDX-License-Identifier: MIT

package i18n

// bgPolishKeys holds English strings added by the 2026-07-19 Budgets/Goals
// first-viewport polish: the Budgets "Needs attention" top-problems strip and the
// Goals payday-funding review banner. Merged via init so the shared en.go is never
// touched by this concurrent lane.
var bgPolishKeys = Catalog{
	// --- Budgets: "Needs attention" strip (top three problem categories) ---
	"bgpolish.attnTitle": "Needs attention",
	// %d = number of budgets shown in the strip.
	"bgpolish.attnCount":    "%d of your budgets need a look right now.",
	"bgpolish.attnCountOne": "One budget needs a look right now.",
	"bgpolish.attnOver":     "Over",
	"bgpolish.attnNear":     "Near limit",
	"bgpolish.attnPace":     "Trending over",
	// %s ×2 = spent, limit (e.g. "$120 of $100").
	"bgpolish.attnSpentOf": "%s of %s",
	// %s = the amount a budget is over by.
	"bgpolish.attnOverBy": "%s over",
	"bgpolish.attnView":   "View spending",
	// %s = the budget/category name.
	"bgpolish.attnViewTitle": "Open the transactions behind %s",

	// --- Goals: collapsible payday-funding review banner ---
	// %s = the amount of fresh income ready to fund goals.
	"bgpolish.wfBannerTitle":    "Review your funding plan",
	"bgpolish.wfBannerReady":    "%s ready to fund",
	"bgpolish.wfBannerExpand":   "Show the funding plan",
	"bgpolish.wfBannerCollapse": "Hide the funding plan",
}

func init() {
	for k, v := range bgPolishKeys {
		english[k] = v
	}
}
