// SPDX-License-Identifier: MIT

package i18n

// budgetGuardKeys holds the English copy for the /budgets guard rails added by the
// 2026-08-17 recheck: limit validation that runs while you type rather than only on
// commit (C665/C666), the top-up dialog's base-limit-vs-effective-cap arithmetic
// (C669), and the compact-list toggle's stateful accessible name (C670). Kept in its
// own file and merged via init so it never touches the shared en.go.
var budgetGuardKeys = Catalog{
	// ── Limit validation (C665, C666) ────────────────────────────────────────
	// Named by the LimitProblem the pure parser reports, so every editor that
	// takes a limit says the same thing about the same input.
	"budgets.limitBlank":       "Enter a limit.",
	"budgets.limitPositive":    "Enter an amount greater than %s.", // formatted zero in the budget's currency
	"budgets.limitMalformed":   "That isn't an amount — enter digits, like 250.00.",
	"budgets.limitSaveBlocked": "Fix the limit to save.",

	// Add-budget required fields + what is still missing (C666).
	// The create-a-new-category path needs no rule of its own: a blank category
	// name falls back to the budget's name, so requiring the name covers both.
	"budgets.fieldRequired": "(required)",
	"budgets.addNeedsName":  "Give this budget a name to continue.",
	"budgets.addNeedsLimit": "Enter a limit to continue.",
	"budgets.addNeedsBoth":  "Give this budget a name and a limit to continue.",

	// ── Top-up: base limit vs effective cap (C669) ───────────────────────────
	"budgets.topupHintPlain": "Increase %s by:",
	// math already reads "$500.00 limit + $221.71 carried in"; effective cap follows.
	"budgets.topupCapMath":      "This period's cap: %s = %s.",                                                                                         // math, effective cap
	"budgets.topupCapPlain":     "Base limit %s — nothing is carried in or added, so that is this period's cap too.",                                   // base limit
	"budgets.topupChangesMonth": "Your amount is added to this period's cap of %s. The base limit stays at %s, and the cap reverts to it next period.", // effective cap, base limit
	"budgets.topupChangesPerm":  "Your amount is added to the base limit of %s, so every future period starts from the higher number.",                 // base limit

	// ── Compact-list toggle (C670) ───────────────────────────────────────────
	// A screen-reader-only suffix on the button's stable visible label, so the
	// accessible name carries both the current view and what a click produces
	// without the label contradicting aria-pressed (the C596 failure).
	// The visible label names where a click GOES, so the accessible name is what
	// carries where you ARE. Without this a screen-reader user reading "Full cards"
	// has no way to tell whether that is the current view or the destination.
	"budgets.densityStateOn":  "showing the compact list",
	"budgets.densityStateOff": "showing full cards",
}

func init() {
	for k, v := range budgetGuardKeys {
		english[k] = v
	}
}
