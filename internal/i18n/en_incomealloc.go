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
	"budgets.allocNoBudgets":  "No budgets yet — none of your %s income has a job.",
	"budgets.allocBudgets":    "Budgets",
	"budgets.allocUnassigned": "Not yet budgeted",
	"budgets.allocOverShort":  "Over income",
	"budgets.allocMarker":     "Where your income runs out",
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
