// SPDX-License-Identifier: MIT

package i18n

// coverAllKeys holds the copy for the "Cover overages in one pass" Smart+ feature
// (SMART-B14): the over-banner button and the shared flip modal that clears every
// over-budget at once — each covered from another budget's slack or borrowed from
// next month's same budget. Kept in its own file, merged via init.
var coverAllKeys = Catalog{
	// Over-banner entry point.
	"coverAll.button": "Cover all",

	// Modal shell.
	"coverAll.title": "Cover budget overages",
	// %d = number over, %s = total overage amount.
	"coverAll.intro":    "%d budgets are over by %s total. Choose how to cover each, then apply them together.",
	"coverAll.introOne": "1 budget is over by %s. Choose how to cover it.",
	"coverAll.none":     "No budgets are over this period — nothing to cover.",

	// One over-budget row.
	"coverAll.overBy":      "over by %s",
	"coverAll.sourceLabel": "Cover from",

	// Coverage sources.
	"coverAll.sourceSkip":      "Leave uncovered",
	"coverAll.sourceNextMonth": "Next month's budget",
	// %s = source budget name, %s = amount still available in it this period.
	"coverAll.sourceBudget":  "%s — %s left",
	"coverAll.nextMonthHint": "Borrows from next month's same budget: this month goes up, next month goes down by the same amount.",

	// Actions + results.
	"coverAll.apply":  "Cover all",
	"coverAll.cancel": "Cancel",
	// %d = how many overages were covered.
	"coverAll.doneOne":  "Covered 1 overage.",
	"coverAll.doneMany": "Covered %d overages.",
	"coverAll.doneNone": "Nothing selected to cover.",
	// %s = source budget name, %s = shortfall it couldn't cover.
	"coverAll.sourceShort": "%s doesn't have %s of slack left this period.",
}

func init() {
	for k, v := range coverAllKeys {
		english[k] = v
	}
}
