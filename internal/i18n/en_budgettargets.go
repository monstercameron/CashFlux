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
	"budgets.fillHeading":     "Quick fill from history",
	"budgets.fillLastMonth":   "Last month",
	"budgets.fillAvg3":        "Avg 3 mo",
	"budgets.fillAvg6":        "Avg 6 mo",
	"budgets.fillLastPeriod":  "Last period",
	"budgets.fillUnderfunded": "To target",
	"budgets.fillApply":       "Set the amount to %s (%s)", // label, value
}

func init() {
	for k, v := range budgetTargetKeys {
		english[k] = v
	}
}
