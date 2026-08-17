// SPDX-License-Identifier: MIT

package i18n

// budgetTargetKeys holds the English strings for per-budget funding targets (BG1)
// and the quick-budget fill chips (BG4). Kept in its own file and merged via init
// so it never touches the shared en.go.
var budgetTargetKeys = Catalog{
	// Target editor (budget edit form).
	"budgets.targetLabel":         "Funding target",
	"budgets.targetHint":          "Optionally fund this budget toward a goal, not just a limit.",
	"budgets.targetNone":          "No target",
	"budgets.targetRefill":        "Refill up to an amount each period",
	"budgets.targetSetAside":      "Set aside a fixed amount each period",
	"budgets.targetByDate":        "Save a set amount by a date",
	"budgets.targetAmountLabel":   "Target amount",
	"budgets.targetDateLabel":     "Target date",
	"budgets.targetLinkGoalLabel": "Link a goal (optional)",
	"budgets.targetLinkGoalNone":  "No linked goal",
	"budgets.targetLinkGoalHint":  "A dated target can borrow its pace from a savings goal so the math lives in one place.",

	// Target summary lines (budget row). %s order noted per key.
	"budgets.targetRefillRow":   "Refill to %s · %s to go",     // target, needed
	"budgets.targetRefillMet":   "Refill to %s · fully funded", // target
	"budgets.targetSetAsideRow": "Set aside %s each period",    // target
	"budgets.targetByDateRow":   "%s by %s · %s to go",         // target, date, needed
	"budgets.targetByDateMet":   "%s by %s · on track",         // target, date

	// Quick-fill chips (budget edit form).
	//
	// C667: the chips name what each figure IS. Three of them are spending and one
	// is a plan, and calling the prior LIMIT "Last period" in a row of spend figures
	// invited a budget to be rewritten to its own old number. The heading and the
	// explanation line say which categories and which periods the history covers,
	// because those have to be the ones the card's spending bar counts.
	"budgets.fillHeading":     "Quick fill",
	"budgets.fillLastPeriod":  "Last period spend",
	"budgets.fillAvg3":        "Avg spend · 3 periods",
	"budgets.fillAvg6":        "Avg spend · 6 periods",
	"budgets.fillPriorLimit":  "Prior limit",
	"budgets.fillUnderfunded": "To target",
	// Explanation under the chips: cadence, window, and population.
	"budgets.fillExplain":     "Spend figures cover %d whole %s periods (%s – %s), counted from the same categories and tags as this budget's spending bar.", // periods, cadence, from, to
	"budgets.fillExplainNone": "Spending history isn't available for this budget right now — only the prior limit is offered.",
	"budgets.fillLimitNote":   "“%s” is the budget that was set last period, not money that was spent.", // prior-limit label
	"budgets.fillKindSpend":   "actual spend",
	"budgets.fillKindLimit":   "a previous limit, not spend",
	"budgets.fillKindTarget":  "what this budget's target still needs",
	"budgets.fillApplyKind":   "Set the amount to %s — %s (%s)", // label, kind, value
}

func init() {
	for k, v := range budgetTargetKeys {
		english[k] = v
	}
}
