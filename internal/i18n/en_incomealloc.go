// SPDX-License-Identifier: MIT

package i18n

// incomeAllocKeys holds the copy for the /budgets income-allocation read: the
// answer to "how much of my income have I budgeted?", which the hero band never
// gave because it only ever compared spending against the budget.
//
// Wording rules applied here, after an adversarial pass on the first draft:
//
//   - The denominator sits IMMEDIATELY beside the percentage. The first draft put
//     "162%" on the left of the caption and "of $5,900.00 income" a thousand
//     pixels away at the right edge, so the reader had to hold one fragment in
//     mind while hunting for the other.
//   - A closed month is described in the past tense. "More than you earn" over a
//     month that ended is a claim about an ongoing state that no longer exists.
//   - The figure is labelled for what it actually is. When last month's leftover
//     rolls in, the denominator is not income — it is income plus rollover — and
//     calling it "income" puts a wrong number under a confident label.
//   - No cheerleading. "Every dollar has a job" was the one line that read like
//     marketing dropped into a data readout.
//
// Merged via init so the shared en.go is untouched by this lane.
var incomeAllocKeys = Catalog{
	"budgets.allocCap":     "Budgeted against income",
	"budgets.allocCapHist": "Budgeted against income that month",
	// %d = the plan as a percentage of income. Can exceed 100.
	"budgets.allocPct": "%d%%",
	// The denominator, rendered directly beside the percentage. %s = the figure.
	// "available" is used when last month's leftover is part of the pool, because
	// then the number is not income alone.
	"budgets.allocOfIncome":    "of your %s income",
	"budgets.allocOfAvailable": "of your %s available (income + rollover)",
	// The relationship, present tense for a live month.
	"budgets.allocUnder": "%s still to budget",
	"budgets.allocExact": "fully budgeted",
	"budgets.allocOver":  "%s more than you earn",
	// ...and past tense for a month that has closed.
	"budgets.allocUnderHist": "%s went unbudgeted",
	"budgets.allocExactHist": "fully budgeted",
	"budgets.allocOverHist":  "%s more than was earned",
	// First-run: an income basis is set but no budgets exist yet. This is the very
	// first thing a household sees after configuring income, so it reads as an
	// invitation rather than a deficiency.
	"budgets.allocNoBudgets": "No budgets yet — none of your %s income has a job.",
	// Named by DIRECTION, not by noun. A spending budget is a CEILING (stay under
	// $300 for Dining); a savings assignment is a FLOOR (put $500 in the HYSA).
	// They are inverses — going over one is bad, going over the other is good — and
	// drawing both as "assigned" slices of one bar is arithmetically right and
	// directionally mute. The words are the only place that distinction can live
	// (Cam, 2026-08-31).
	"budgets.allocBudgets": "Spending limits",
	// Shown in the legend when zero-based has no savings assigned yet: the slot is
	// stated rather than omitted, because absence is not information.
	"budgets.allocSavingsNone": "none set",
	"budgets.allocUnassigned":  "Not yet budgeted",
	"budgets.allocOverShort":   "Over income",
	"budgets.allocMarker":      "Where your income runs out",
	// Screen-reader descriptions of the bar. %d = the plan as a percent of income.
	"budgets.allocAria":     "You have budgeted %d%% of your income; the remainder is unbudgeted",
	"budgets.allocAriaOver": "You have budgeted %d%% of your income — the tick marks where your income runs out, and the fill past it is over-budgeted",
	// The control that opens the income-basis picker. It names its object: a bare
	// "Change" beside five other elements answers "change what?" with nothing.
	"budgets.allocChange": "Change income",
	// The caveat pinned under the hero's "Left" figure when the plan is over
	// income, so the biggest number on the page cannot be read as slack that
	// exists. %s = how far the plan runs past earnings.
	"budgets.leftInBudget": "Left in budget",
	// The C535 opt-in on the add-a-budget form. The label says what ticking it
	// DOES, and the forced note says why it cannot be unticked on a household
	// that has no categories to attach a budget to.
	"budgets.createCatOptIn": "Create a new category for this budget",
	// Two or more categories already hold this name at different levels, so which
	// one was meant is genuinely unclear. Say so rather than guessing.
	// %d = how many already hold it, %s = the name.
	"budgets.catAmbiguous":       "%d categories are already called “%s” — a new top-level one will be created.",
	"budgets.createCatForced":    "You have no spending categories yet, so this budget will create one.",
	"budgets.leftOverIncomeNote": "but %s over income",
	// C520: the direction control on the transaction edit form. The amount field
	// holds a magnitude and this holds the sign, so a charge filed as income can
	// be corrected to a spend instead of having to be deleted and re-entered.
	"transactions.directionLabel": "Direction",
	// C523: merging two categories into one. The verbs say what happens to the
	// data ("move"), not what happens to the record ("delete"), because moving
	// the data is the point and the retirement is a consequence.
	"categories.mergeMenu":      "Merge into…",
	"categories.mergeMenuTitle": "Fold this category into another one, moving everything that points at it",
	// %s = the category being folded away.
	"categories.mergeTitle":   "Merge “%s” into",
	"categories.mergeDesc":    "Everything filed under it moves to the category you pick, and it stops existing. Nothing is deleted.",
	"categories.mergeConfirm": "Merge",
	// C538: which way money has to move for a budget to be doing well.
	"budgets.directionLabel":    "What this budget measures",
	"budgets.directionSpend":    "Spending — stay under the amount",
	"budgets.directionSave":     "Saving — reach the amount",
	"budgets.directionSaveHint": "Money you move into savings or investments counts toward it, including transfers between your own accounts.",
	// C524: the per-category spend report already exists in full — a magnitude
	// histogram, prior-period deltas, sparklines and a drill-through per bar.
	// These point at it rather than duplicating it into a second set of numbers.
	"budgets.seeSpend": "See where it went",
	// Shown when a note save matched no budget. Silence on a write is worse than
	// an error: the note vanishes and nothing says why.
	"budgets.notesSaveFailed": "Could not save the note — this budget no longer exists. Copy your text before closing.",
	// Shown when a write is rejected by a layer that does not describe itself in
	// words a person can act on. The technical text goes to the log instead: on
	// screen it would arrive at the moment someone most needs to understand what
	// happened, and say nothing they can do about it.
	"budgets.writeFailed":          "That change could not be saved, so nothing was changed. Try again — if it keeps happening, reload the page.",
	"budgets.writeReadOnly":        "This household is open in a viewing role, so budgets cannot be changed here.",
	"budgets.flexTargetUnreadable": "That is not an amount we can read. Try a number like 2,400 or 2400.50.",
	// C614: the notes editor names the ACTION, and names it by state. Every other
	// item in the row's ⋯ menu is a verb phrase ("Edit budget", "Delete"); "Notes"
	// was the one noun, and it read the same whether it would create a note or
	// overwrite one. The dropped word "private" was worse than vague — a budget is
	// shared by default and its note carries no per-member access of any kind, so
	// the copy asserted something the data model does not do.
	"budgets.notesAdd":  "Add a note",
	"budgets.notesEdit": "Edit note",
	// Removing a note is now an explicit act. It used to be reachable only by
	// selecting the text, deleting it, and pressing Save — a destructive gesture
	// with no name, which a user had to reverse-engineer.
	"budgets.notesRemove":        "Remove note",
	"budgets.notesRemoveConfirm": "Remove the note on %s? The text is not kept.",
	// Confirmation at the moment of the write. These name the budget because notes
	// are per-budget and several can be written in a row, so a bare "Note saved"
	// leaves you unsure which card it landed on.
	"budgets.notesSaved":   "Note saved on %s",
	"budgets.notesRemoved": "Note removed from %s",
	// C516: a completion that is billed but carries no text - a reasoning-only
	// turn, a length cap hit before any visible text, or a filtered response.
	// Rendering nothing made this indistinguishable from the assistant being
	// broken, which is exactly how it was reported.
	"insights.emptyReply":   "The model returned no text this time — the request was charged but produced nothing to show. Try asking again, or rephrase the question.",
	"budgets.seeSpendTitle": "Spending broken down by category, with last period alongside",
	// C521: the tracked-categories page inside the budget editor.
	"budgets.editCatsBack":         "← Back to budget",
	"budgets.editCatsBackHint":     "Your other changes are still here.",
	"categories.mergeOpen":         "Merge categories…",
	"categories.mergeTitleAny":     "Merge one category into another",
	"categories.mergeChooseSource": "Choose the category to fold away",
	// %d = total references, %s = the surviving category.
	"categories.mergePreview": "Moves %d things into %s.",
	// %s = source, %s = destination, %d = how many references moved.
	"categories.mergedToast":        "Merged %s into %s — %d references moved.",
	"categories.mergePartTxns":      "%d transactions",
	"categories.mergePartSplits":    "%d split lines",
	"categories.mergePartBudgets":   "%d budgets",
	"categories.mergePartGoals":     "%d goals",
	"categories.mergePartRules":     "%d rules",
	"categories.mergePartRecurring": "%d recurring",
	"categories.mergePartChildren":  "%d sub-categories",
	"categories.mergePartNothing":   "(nothing points at it yet)",
	// C517: the direction filter on /transactions. The Criteria field and its chip
	// already existed and were only reachable through natural-language search.
	"transactions.flowLabel": "Money in or out",
	"transactions.flowAny":   "Both",
	"transactions.flowOut":   "Money out (spending)",
	"transactions.flowIn":    "Money in (income)",
	// C519: a spending budget tracks spending categories. Saying so beats leaving
	// the reader to conclude the list is broken. %d = how many were withheld.
	"budgets.catsIncomeHidden":  "%d income categories are not shown — a spending budget tracks what you spend, not what you earn.",
	"transactions.directionOut": "Money out (spending)",
	"transactions.directionIn":  "Money in (income)",
}

func init() {
	for k, v := range incomeAllocKeys {
		english[k] = v
	}
}
