// SPDX-License-Identifier: MIT

package i18n

// budgetsHeroKeys holds English strings added by the 2026-07-19 /budgets "one
// answer" hero (B1): the page opens on the left-to-spend figure with a slim
// month-ledger bar and a single caption, replacing the three-cell status strip.
// Merged via init so the shared en.go is never touched by this concurrent lane.
var budgetsHeroKeys = Catalog{
	// The hero fig's label states tense + state; the value is only ever an amount
	// ("Jun 2026 · Over budget / $509.58" — never "Unspent / $509.58 over").
	"budgets.heroOverLabel": "Over budget",
	// The attention chip beside the hero number; clicking narrows the list.
	"budgets.heroAttn":    "%d need attention",
	"budgets.heroAttnOne": "1 needs attention",
	// Chip title: what a click does.
	"budgets.heroAttnTitle": "Show only the budgets that are over or near their limit",
	// Zero-based only: the still-unassigned pool, one small chip. %s = amount.
	"budgets.heroToAssign": "To assign: %s",
	// Zero-based hero labels (2026-08-31). To Assign is the method's ONE governing
	// number, so it IS the hero figure and it is named by its state — the value
	// beside them is always an unsigned amount, matching heroOverLabel's contract.
	// Zero is the success state in zero-based: a finished plan assigns every dollar,
	// so "All assigned" reads as done rather than as an empty pool.
	"budgets.heroToAssignLabel": "To assign",
	"budgets.heroOverAssigned":  "Over-assigned",
	"budgets.heroAllAssigned":   "All assigned",
	// The hero's help tip in zero-based. The default "safe to spend" tip describes
	// a per-category remainder, which is not what this number is.
	"smart.tipBudgetToAssign": "Income you have not assigned yet. In zero-based budgeting the goal is zero — every dollar given a job, across your budgets and your savings.",
	// The spend rail under the allocation bar, and the caption naming both ends of
	// it. %s spent, %s assigned — the pair the loader band used to carry as figures.
	"budgets.spendOfAssigned": "%s spent of %s assigned",
	// …and where any of that is a savings target, say so. The two allocations
	// behave in opposite directions, so a single "assigned" figure hides which part
	// of it you are meant to stay UNDER and which part you are meant to reach.
	"budgets.spendOfAssignedSplit": "%s spent of %s assigned · %s of that to savings",
	"budgets.spendRailAria":        "Spent: %d%% of the money assigned",
	// Savings & investments fold. The hint states what is inside so the closed row
	// is still informative — %s is the monthly total already assigned.
	// Plural pairs. TN prepends the count, so the reading order is preserved with
	// explicit argument indexes rather than by rewording around the verb.
	"budgets.savingsHintOne":  "%[2]s a month across %[1]d account",
	"budgets.savingsHintMany": "%[2]s a month across %[1]d accounts",
	"budgets.savingsHintNone": "Set a monthly amount per account",
	"budgets.savingsShowAria": "Show savings targets",
	"budgets.savingsHideAria": "Hide savings targets",
	// Recurring's collapsed line: the committed monthly outflow and how many charges
	// make it up. That pair is what the open section exists to total, so the closed
	// row answers the question and the open one shows the working.
	"budgets.recurring.foldHintOne":  "%[2]s a month · %[1]d charge",
	"budgets.recurring.foldHintMany": "%[2]s a month · %[1]d charges",
	"budgets.recurring.showAria":     "Show the recurring charges feeding these budgets",
	"budgets.recurring.hideAria":     "Hide the recurring charges",
	// Plan the year's collapsed line. %d is the year, %s the plan's annual total.
	"budgets.planYearFoldHint": "%d · %s planned",
	// The compact row's coverage marker. The card has carried this since cover
	// existed; the list never did, so a budget that only looks healthy because
	// another one is paying for it read as plain healthy.
	//
	// The pill says COVERED and nothing more — the interesting part is whether it
	// is a standing arrangement or a one-off, and by how much, and that is what the
	// popover is for. A pill long enough to say all of it would not fit the cell.
	"budgets.coverMarkerPill":         "Covered",
	"budgets.coverMarkerOngoingTitle": "Covered every period",
	// %s = the amount moved in, %d = how many budgets it comes from. The figure is
	// what makes the marker worth opening: "covered" alone does not say whether
	// another budget is quietly carrying $10 of this one or $400.
	"budgets.coverMarkerOngoingTextOne":  "%[2]s moves into this budget from %[1]d other budget at the start of every period. It keeps covering until you remove the arrangement.",
	"budgets.coverMarkerOngoingTextMany": "%[2]s moves into this budget from %[1]d other budgets at the start of every period. It keeps covering until you remove the arrangement.",
	"budgets.coverMarkerOngoingPlain":    "This budget is topped up from others at the start of every period.",
	"budgets.coverMarkerOnceTitle":       "Covered this period",
	"budgets.coverMarkerOnceText":        "This budget was topped up from other budgets this period only. Next period it starts at its own limit again.",
}

func init() {
	for k, v := range budgetsHeroKeys {
		english[k] = v
	}
}
