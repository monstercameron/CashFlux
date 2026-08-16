// SPDX-License-Identifier: MIT

package i18n

// budgetsClarityKeys holds English strings added by the C584–C598 Budgets
// correctness + clarity pass (2026-08-16): making a budget's drill-through match
// its own total, naming the scope of every control, and turning the page's
// funds-moving actions into forms that state what they will do. Merged via init
// so the shared en.go is never touched by this lane.
var budgetsClarityKeys = Catalog{
	// --- C591: one labelled drill-through per budget ---
	// The budget name used to navigate silently; the action is now always a
	// labelled control. %s = the budget's title.
	"budgets.drillAria": "See the transactions in %s",
	// The tooltip is the same in both densities and names the BUDGET, never its
	// primary category — a multi-category or tag-tracking budget has no single
	// one, and the old wording interpolated that empty string. %s = the budget.
	"budgets.drillTitlePlain":   "See the transactions in %s for this period",
	"budgets.drillTitleSub":     "See the transactions in %s for this period, including its sub-categories",
	"budgets.drillTitleTags":    "See the transactions in %s for this period, including its tagged charges",
	"budgets.drillTitleSubTags": "See the transactions in %s for this period, including its sub-categories and tagged charges",

	// --- C585: say what the drill-through actually covers ---
	// Shown on the ledger when it was opened from a budget, so a user can tell
	// whether the rows they are looking at are the whole story. %s = the budget.
	"budgets.scopeNoteTitle":      "Showing what %s counts",
	"budgets.scopeNoteCats":       "%s and its sub-categories",
	"budgets.scopeNoteCatsPlain":  "%s",
	"budgets.scopeNoteTags":       "plus anything tagged %s",
	"budgets.scopeNoteSplitsHint": "Split receipts appear when any of their lines is in scope.",
	"budgets.scopeEmptyContra":    "This budget shows %s spent, but no transaction in its scope matches the other filters you have on.",
	"budgets.scopeEmptyClear":     "Clear the other filters",

	// --- C585: the ledger names the budget it was opened from ---
	// A pre-chip ahead of the "Filtering by" sentence, wording chosen for what
	// the budget's scope actually reaches. %s = the budget's title.
	"transactions.budgetDrillChip":        "From the %s budget",
	"transactions.budgetDrillChipSub":     "From the %s budget, including its sub-categories",
	"transactions.budgetDrillChipTags":    "From the %s budget, including its tagged charges",
	"transactions.budgetDrillChipSubTags": "From the %s budget, including its sub-categories and tagged charges",
	"transactions.budgetDrillChipRemove":  "Show all transactions again",

	// --- C589: custom range as an explicit workflow, not a mode flag ---
	// The draft range is previewed in words and applied deliberately. %s = start
	// label, %s = end label, %d = how many units of the current granularity.
	"resolution.rangePreview":      "%s through %s — %d periods",
	"resolution.rangePreviewOne":   "%s only — a single period",
	"resolution.rangeScopeNote":    "A range changes what these pages report on. Each budget and goal keeps its own cadence.",
	"resolution.rangeApply":        "Apply range",
	"resolution.rangeDiscard":      "Discard changes",
	"resolution.rangeBackToSingle": "Back to a single period",
	"resolution.rangeClose":        "Close",

	// --- C586: an estimate only when there is history to estimate from ---
	"budgets.suggestNoHistory": "No history yet — this category is new, so there is nothing to average.",
	"budgets.suggestScoped":    "Averaged from %s over the last 6 months.",
}

func init() {
	for k, v := range budgetsClarityKeys {
		english[k] = v
	}
}
