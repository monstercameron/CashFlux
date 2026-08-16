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

	// --- C607: supporting modules report the period the page is showing ---
	// They used to read today's month and label it "this month" whatever period
	// was selected, so a closed July view listed August's figures. %s = the
	// period label, e.g. "Jul 2026".
	"budgets.trackMetaHintPeriod":      "Figures cover %s — transactions · total.",
	"budgets.unbudgetedHintPeriod":     "Categories with spending in %s that no budget tracks.",
	"budgets.unbudgetedHintPeriodPast": "Categories that had spending in %s and no budget tracking them.",
	"budgets.unbudgetedThisPeriod":     "this period",

	// --- C597: every funds-moving action explains its reach in the same words ---
	// Assembled by screens.fundsImpactLine. Kept as fragments so six flows read
	// identically instead of each inventing its own phrasing.
	"budgets.impactThisPeriodOnly": "Changes this period only — next period is unaffected.",
	"budgets.impactThisAndFuture":  "Changes this period and every one after it.",
	"budgets.impactFutureOnly":     "Changes future periods; this one is unaffected.",
	"budgets.impactOtherBudgets":   "Money comes out of the budgets you pick.",
	"budgets.impactNoRealMoney":    "No account balances change — this is the plan, not the money.",
	"budgets.impactReversible":     "You can undo this.",
	// Delete: the transactions survive, which is the part people fear losing.
	"budgets.deleteConfirmHonest": "Delete the \"%s\" budget? Its transactions stay exactly where they are — they just stop counting against a cap.",

	// --- C609: a recurring date says what it is relative to ---
	// "Next Jul 3, 2026" beside "Next Sep 1, 2026" said nothing about whether
	// either had happened. %s = the formatted date.
	"budgets.recurring.overdue":     "Was due %s",
	"budgets.recurring.dueInPeriod": "Due %s",
	"budgets.recurring.afterPeriod": "Next due %s, after this period",

	// --- C606: funding a cover or top-up shows what can really be moved ---
	// The lists used to print a source's whole LIMIT under "available" — the size
	// of its plan, most of which was already spent. %s = the movable amount.
	"budgets.coverMovable": "%s can move",
	// %s = the committed amount, %s = what is genuinely free after it.
	"budgets.coverCommittedNote": "%s of that is already committed this period — %s is free",
	// %s = what the source would have left once its share is taken.
	"budgets.coverAfterNote": "→ %s left after this",
	// %s = the amount the chosen sources fall short by.
	"budgets.coverShortBy": "The budgets you picked are %s short of that. Choose more sources, or lower the amount.",

	// --- C593: Auto budget shows its financial impact and can be edited in bulk ---
	// %s = a count phrase like "6 categories".
	"budgets.autoGroupNew":      "New budgets · %s",
	"budgets.autoGroupExisting": "Already budgeted · %s — ticking these overwrites the limit you set",
	"budgets.autoSelectAll":     "Select all",
	"budgets.autoSelectNone":    "Select none",
	// %s = a count phrase like "3 sliders".
	"budgets.autoResetTuning": "Reset %s to the suggestion",
	"budgets.autoImpactLabel": "Budgeted each month",
	// %s = the amount left, %s = the income it is measured against.
	"budgets.autoImpactLeft": "leaves %s of your %s income",
	"budgets.autoImpactOver": "%s MORE than your %s income",

	// --- C588: the plan rail names its contents and its destinations ---
	// The header used to read "2 items to review from this period", which sounds
	// like a transaction inbox and hides three different destinations. %s = the
	// joined list of what is actually in there.
	"common.and":                    "and",
	"budgets.issuesRailNamed":       "%s to sort out",
	"budgets.issuesRailHist":        "%s left from this period",
	"budgets.issuePartOverAssigned": "an over-assignment",
	"budgets.issuePartFundShort":    "a sinking-fund shortfall",
	"budgets.issuePartFollowUps":    "open follow-ups",
	// Each row's button names the PAGE it opens, so the destination is readable
	// before the click rather than discovered after it.
	"budgets.issueGoAllocate": "Open Allocate",
	"budgets.issueGoTodo":     "Open To-do",
	"budgets.issueGoGoals":    "Open Goals",

	// --- C590: which method control governs what ---
	// The household picker names its own scope; the per-budget one says where the
	// budget currently gets its method from and that a choice here stops with it.
	"budgets.methodGlobalLabel":         "Budgeting method (whole household)",
	"budgets.methodGlobalTitle":         "How budgeting works across this page. Individual budgets can set their own.",
	"budgets.methodScopeNone":           "Applies to every budget you add.",
	"budgets.methodScopeAll":            "Applies to all %s here, and to every budget you add.",
	"budgets.methodScopeMixed":          "Applies to the %s following it. %s set their own method and will not change.",
	"budgets.methodGlobalConfirm":       "Switch the household to %s? This changes the %s following the household method. %s set their own and will keep it.",
	"budgets.methodGlobalConfirmAction": "Switch the household",
	"budgets.methodGlobalSaved":         "Household method is now %s — %s changed.",
	"budgets.methodOwnFollowing":        "Following the household method (%s). Choosing another applies to this budget only.",
	"budgets.methodOwnOverriding":       "This budget uses %s regardless of the household method (%s).",

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
