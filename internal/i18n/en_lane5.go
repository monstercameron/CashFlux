// SPDX-License-Identifier: MIT

package i18n

// lane5Keys holds English strings added by the 2026-07-17 goal/budget/household
// refinement lane (#51 slider accessibility, #70 budgets historical wording,
// #71 compact goal cards, #64 month close, #65 goals refinement, #66 household
// clarity). Merged via init so this file never touches the shared en.go.
var lane5Keys = Catalog{
	// #51 — the contribution planner's direct numeric entry.
	"goals.planAmountLabel": "Monthly contribution amount",

	// #70 (UX-05) — budgets: historical-period wording, explainable counts, Automate menu.
	"budgets.histSpendCap":      "%s spending",
	"budgets.histUnspent":       "Unspent",
	"budgets.histIssuesRail":    "%d items to review from this period",
	"budgets.histIssuesRailOne": "1 item to review from this period",
	"budgets.histOverBanner":    "%d categories ended over budget by %s total.",
	"budgets.histOverBannerOne": "1 category ended over budget by %s.",
	"budgets.histNearBadge":     "%d finished near the limit",
	"budgets.followUpsCount":    "%d unresolved follow-ups",
	"budgets.followUpsCountOne": "1 unresolved follow-up",
	"budgets.followUpsRowBody":  "To-dos linked to these budgets that are still open.",
	"budgets.followUpsRowView":  "View to-dos",
	"budgets.filterShow":        "Show in list",
	"budgets.filterOverTitle":   "Filter the list to the over-budget categories",
	"budgets.filterNearTitle":   "Filter the list to the categories near their limit",
	"budgets.attentionOver":     "Showing over-budget categories only",
	"budgets.attentionNear":     "Showing near-limit categories only",
	"budgets.attentionClear":    "Show all",
	"budgets.automate":          "Automate",
	"budgets.automateTitle":     "Bulk budget tools — last month's spend, auto budget, sweep leftovers, adjust all",
	"budgets.followUpsShow":     "Show the follow-ups",
	"budgets.followUpsHide":     "Hide the follow-ups",

	// #64 — the guided month-close flow.
	"monthclose.title":           "Close out %s",
	"monthclose.intro":           "A quick walk through %s's loose ends — nothing here is required, and nothing happens without your say-so.",
	"monthclose.offer":           "Close out the month",
	"monthclose.offerTitle":      "Review overspending, unused money, and next month's plan in one guided pass",
	"monthclose.overTitle":       "1 · Overspending",
	"monthclose.overNone":        "Nothing ended over budget. Nice.",
	"monthclose.overIntro":       "%d categories went over by %s total.",
	"monthclose.coverAction":     "Review & cover",
	"monthclose.overLeaveNote":   "Or leave them — next month simply starts honest.",
	"monthclose.leftTitle":       "2 · Unused money",
	"monthclose.leftNone":        "No budget ended with money left over.",
	"monthclose.leftIntro":       "%s went unspent across %d budgets.",
	"monthclose.rolloverOn":      "Leftover rollover is ON: this money joins next month's pool automatically.",
	"monthclose.rolloverOff":     "Leftover rollover is OFF: each budget starts next month fresh at its limit, and this leftover stays wherever it sits.",
	"monthclose.rolloverEnabled": "Leftover rollover is now on — next month's pool will include unspent budget.",
	"monthclose.assignTitle":     "3 · Does the plan fit the income?",
	"monthclose.assignFits":      "Your plan fits the expected income.",
	"monthclose.assignOver":      "The plan claims %s more than the expected income. The honest ways out:",
	"monthclose.reduce":          "Trim a category",
	"monthclose.reduceTitle":     "Open the allocation page and reduce a category with room",
	"monthclose.income":          "Revisit expected income",
	"monthclose.incomeTitle":     "Open the income-basis settings — raise or correct what the plan expects",
	"monthclose.rollover":        "Use rollover",
	"monthclose.rolloverTitle":   "Turn on leftover rollover so unspent budget absorbs the gap",
	"monthclose.defer":           "Leave unresolved",
	"monthclose.deferTitle":      "Acknowledge the gap and move on — it stays visible on the budgets page",
	"monthclose.deferredNote":    "Left unresolved: the plan still claims %s more than expected income. The budgets page keeps showing it.",
	"monthclose.incomeTitle2":    "4 · Income: expected vs actual",
	"monthclose.incomeExpected":  "Expected (your basis)",
	"monthclose.incomeActual":    "Actually received",
	"monthclose.incomeMatched":   "Income landed right on plan.",
	"monthclose.incomeAhead":     "You brought in %s more than the plan expected.",
	"monthclose.incomeBehind":    "You brought in %s less than the plan expected — worth a look before funding next month.",
	"monthclose.copyTitle":       "5 · Carry last period's top-ups",
	"monthclose.copyNone":        "No one-time top-ups last period, so there's nothing to carry — limits themselves carry over automatically.",
	"monthclose.copyIntro":       "These budgets had one-time top-ups last period. Untick any exception, then carry the rest into this period:",
	"monthclose.copyApply":       "Carry %d top-ups",
	"monthclose.copyApplyTitle":  "Write the ticked top-ups as this period's one-time boosts (undoable)",
	"monthclose.copyApplied":     "Carried last period's top-ups into %s.",
	"monthclose.done":            "Done",
	"budgets.incomeActualSoFar":  "Received so far: %s of the %s your plan expects.",
	"budgets.incomeActualEnded":  "Received: %s against the %s the plan expected.",

	// #65 — goals refinement: plan comparison, conflicts, paycheck preview, funding order.
	"goals.compareEasier":         "Take it easier (−25%)",
	"goals.compareYours":          "Your plan",
	"goals.compareHarder":         "Push harder (+25%)",
	"goals.compareNoLanding":      "no landing date",
	"goals.conflictTitle":         "%s all set money aside from %s",
	"goals.conflictBody":          "Together they claim %s but the account holds %s — %s more than it can back.",
	"goals.conflictReview":        "Review earmarks",
	"goals.conflictReviewTitle":   "Open the earmarks manager to adjust who claims what",
	"goals.paycheckPreviewToggle": "Preview: what would a %s paycheck fund?",
	"goals.paycheckPreviewIntro":  "If your next paycheck lands around %s (your largest recent one), the waterfall would set aside:",
	"goals.paycheckPreviewNote":   "This is only a preview — when the paycheck actually arrives, the same plan is offered for your approval.",
	"goals.fundingOrderToggle":    "Funding order",
	"goals.fundingOrderIntro":     "Payday money fills these goals top to bottom. Reorder to change who gets funded first.",
	"goals.fundingMoveUp":         "Fund %s earlier",
	"goals.fundingMoveDown":       "Fund %s later",

	// #71 (UX-06) — the compact goal card's expand/collapse control.
	"goals.expand":        "Details",
	"goals.expandTitle":   "Show everything on this goal",
	"goals.collapse":      "Less",
	"goals.collapseTitle": "Back to the compact card",
}

func init() {
	for k, v := range lane5Keys {
		english[k] = v
	}
}
