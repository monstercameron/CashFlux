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

	// --- C586: the scoped estimate names the categories it averaged over ---
	// %s = the category path; composed into budgets.suggestScoped below.
	"budgets.scopeNoteCats": "%s and its sub-categories",

	// --- C585: the ledger names the budget it was opened from ---
	// A pre-chip ahead of the "Filtering by" sentence, wording chosen for what
	// the budget's scope actually reaches. %s = the budget's title.
	"transactions.budgetDrillChip":        "From the %s budget",
	"transactions.budgetDrillChipSub":     "From the %s budget, including its sub-categories",
	"transactions.budgetDrillChipTags":    "From the %s budget, including its tagged charges",
	"transactions.budgetDrillChipSubTags": "From the %s budget, including its sub-categories and tagged charges",
	"transactions.budgetDrillChipRemove":  "Show all transactions again",

	// --- C592: "Adjust all" is a form with a preview, not a bare prompt ---
	"budgets.adjustAllTitleHeading": "Adjust every budget",
	"budgets.adjustAllIntro":        "Raise or lower every budget's limit by the same percentage. Nothing changes until you apply it.",
	"budgets.adjustAllFieldLabel":   "Change each limit by",
	// %s = the accepted minimum, %s = the accepted maximum.
	"budgets.adjustAllFieldHint":       "A positive number raises every limit, a negative one lowers it — 5 for 5%% more, -10 for 10%% less. Between %s%% and %s%%.",
	"budgets.adjustAllNotANumber":      "Enter a number, like 5 or -10.",
	"budgets.adjustAllZero":            "0% would leave every budget exactly as it is.",
	"budgets.adjustAllOutOfRange":      "Enter something between %s%% and %s%%. For a bigger change, apply it twice.",
	"budgets.adjustAllNothingToChange": "None of your budgets has a limit to scale yet.",
	"budgets.adjustAllCount":           "%s will change by %s%%.",
	"budgets.adjustAllTotalLabel":      "Total budgeted",
	"budgets.adjustAllDeltaUp":         "+%s",
	"budgets.adjustAllDeltaDown":       "−%s",
	"budgets.adjustAllMixedCurrency":   "These budgets are in different currencies, so there is no single total — each budget's own before and after is listed below.",
	"budgets.adjustAllAckLower":        "I want to lower %s by %s%%. Every one of these plans will have less to spend.",
	"budgets.adjustAllAckRaise":        "I want to raise %s by %s%% — a change this size is usually a typo.",
	"budgets.adjustAllApply":           "Apply to every budget",

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
