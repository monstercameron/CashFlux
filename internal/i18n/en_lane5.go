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
	"monthclose.title":      "Review %s",
	"monthclose.intro":      "A quick walk through %s's loose ends — nothing here is required, and nothing happens without your say-so.",
	"monthclose.offer":      "Review the month",
	"monthclose.offerTitle": "Walk through overspending, unused money, and next month's plan in one pass",
	"monthclose.overTitle":  "1 · Overspending",
	// Live-period twins of the lines below. The flow is offered in a period's last
	// five days, where "went over" / "went unspent" describe an unfinished month as
	// though it had already ended.
	"monthclose.overNoneLive":      "Nothing is over budget so far. Nice.",
	"monthclose.overIntroLiveOne":  "%d budget is over by %s so far.",
	"monthclose.overIntroLiveMany": "%d budgets are over by %s so far.",
	"monthclose.leftNoneLive":      "No budget has money left over right now.",
	"monthclose.leftIntroLiveOne":  "%[2]s is still unspent in %[1]d budget.",
	"monthclose.leftIntroLiveMany": "%[2]s is still unspent across %[1]d budgets.",
	"monthclose.overNone":          "Nothing ended over budget. Nice.",
	"monthclose.overIntroOne":      "%d budget went over by %s total.",
	"monthclose.overIntroMany":     "%d budgets went over by %s total.",
	"monthclose.coverAction":       "Review & cover",
	"monthclose.overLeaveNote":     "Or leave them — next month simply starts honest.",
	"monthclose.leftTitle":         "2 · Unused money",
	"monthclose.leftNone":          "No budget ended with money left over.",
	// Shown when ONE budget is most of the unspent total. %s name, %s amount, %d
	// percent — so the headline figure is not read as money you can move.
	"monthclose.leftDominatedBy": "%s alone accounts for %s of that — %d%%.",
	"monthclose.leftIntroOne":    "%[2]s went unspent in %[1]d budget.",
	"monthclose.leftIntroMany":   "%[2]s went unspent across %[1]d budgets.",
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
	"monthclose.copyTitle":       "5 · Carry top-ups into next period",
	"monthclose.copyNone":        "No one-time top-ups this period, so there's nothing to carry — limits themselves carry over automatically.",
	"monthclose.copyIntro":       "These budgets have one-time top-ups this period. Untick any exception, then carry the rest into the next one:",
	"monthclose.copyApply":       "Carry %d top-ups",
	"monthclose.copyApplyTitle":  "Write the ticked top-ups as next period's one-time boosts (undoable)",
	"monthclose.copyApplied":     "Carried top-ups for %s into next period.",
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

	// #66 — household clarity: roles, ownership, and change previews.
	"members.rolesExplainToggle":   "How roles & ownership work",
	"members.roleOwner":            "Owner",
	"members.roleAdmin":            "Admin",
	"members.roleViewer":           "Viewer",
	"members.roleExplainOwner":     "Runs the household: can see and change everything, manage members and roles, and delete data.",
	"members.roleExplainAdmin":     "A trusted member: can see everything and add or edit accounts, transactions, budgets, and goals — but can't manage members.",
	"members.roleExplainViewer":    "Read-only: can see the household's finances but can't add, change, or delete anything.",
	"members.ownershipExplain":     "Ownership has three separate ideas: an account's OWNER decides whose net worth it counts toward (a person, or Group for shared — optionally split by percentages); a transaction's MEMBER just records who spent; and budgets or goals owned by a person are only theirs. Roles are labels on this shared local dataset, not separate logins.",
	"members.leaveTitle":           "What changes when a member leaves?",
	"members.leavePickPlaceholder": "Pick a member to preview…",
	"members.leaveIntro":           "If %s left, %d things would need a new owner first:",
	"members.leaveNothing":         "%s owns nothing — they could be removed with no reassignment.",
	"members.leaveAccounts":        "%d accounts they own",
	"members.leaveShares":          "%d shared accounts where they hold a percentage",
	"members.leaveBudgets":         "%d budgets",
	"members.leaveGoals":           "%d goals",
	"members.leaveTxns":            "%d transactions tagged with them",
	"members.leaveNote":            "This is only a preview — nothing changes. Deleting the member walks you through reassigning all of it first.",
	"accounts.sharedBadge":         "Shared",
	"accounts.sharedBadgeTitle":    "Owned by the household (or split by percentages), not one person",
	"accounts.ownerMovePreview":    "Saving moves this account's %s of net worth from %s to %s.",

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
