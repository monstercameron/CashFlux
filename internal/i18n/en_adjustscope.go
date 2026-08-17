// SPDX-License-Identifier: MIT

package i18n

// adjustScopeKeys holds the English copy for how long a bulk budget adjustment
// lasts (C671).
//
// "Bring the plan down to what arrived" read as a fix for one underfunded month
// and silently pre-filled a permanent rewrite of every budget. The scope is now a
// choice the form asks and the originating action states, so the wording has to
// name the reach on both sides of the click — before it, on the button that opens
// the form, and inside it, on the control and the confirmation.
//
// Kept in its own file and merged via init so it never touches the shared en.go.
var adjustScopeKeys = Catalog{
	// Which period a this-period change lands on, stated inside the FORM — the
	// toolbar's own "Adjust all" is always available and carries none of the
	// callout's disclosures, so the warning has to live where both entry points
	// pass through.
	"budgets.adjustAllPeriodLive": "You're viewing %s. Each budget's change lands on the period it is in then, which for a weekly or quarterly budget is not the same span.",
	"budgets.adjustAllPeriodPast": "You're viewing %s, which has already ended. Each budget's change lands on the period it is in then, so applying here can change what CashFlux reports about a period that is over.",

	// The scope control inside "Adjust every budget".
	"budgets.adjustAllScopeLabel":     "How long",
	"budgets.adjustAllScopeThis":      "This period only",
	"budgets.adjustAllScopeEvery":     "Every period",
	"budgets.adjustAllScopeThisHint":  "Changes only the period you're looking at. Each budget's own limit is untouched, so next period starts from the plan you already have.",
	"budgets.adjustAllScopeEveryHint": "Rewrites each budget's limit. A limit has no history, so every period — past ones included — is reported against the new number until you change it again.",

	// The acknowledgement. A permanent change is asked about whatever its size,
	// because what makes it consequential is how long it lasts.
	"budgets.adjustAllAckFutureLower": "I want to lower %s by %s%% for EVERY period, not just this one. A budget's limit has no history, so every period is reported against the new number.",
	"budgets.adjustAllAckFutureRaise": "I want to raise %s by %s%% for EVERY period, not just this one. A budget's limit has no history, so every period is reported against the new number.",

	// Applied toasts, so the undo banner says which of the two happened.
	"budgets.adjustAllAppliedThis":  "Adjusted %s by %s%% for this period only.",
	"budgets.adjustAllAppliedEvery": "Adjusted %s by %s%% for this and every future period.",
	"budgets.adjustAllApplyThis":    "Apply to this period",
	"budgets.adjustAllApplyEvery":   "Apply to every period",

	// The originating action on the funding callout: what it does, how far it
	// reaches, and how big it is — before it is pressed.
	//
	// Every variant names the period, and every one takes it as an argument — a
	// verb-free string handed an argument renders Sprintf's %!(EXTRA) garbage, so
	// "this period" is not a shortcut worth taking. On a closed period the BUTTON
	// says so, not just the caption under it: a reader who acts on the prominent
	// control and skips the small grey line beneath is the reader this is for.
	"budgets.fundedReconcileThisPeriod":  "this period",
	"budgets.fundedReconcileScoped":      "Bring %s's plan down to what arrived",
	"budgets.fundedReconcilePast":        "Bring %s's plan down to what arrived — that period has ended",
	"budgets.fundedReconcileTitleScoped": "Scale every budget down so %s's plan matches the money you have — previewed budget by budget, undoable, and set to this period unless you choose otherwise",
	"budgets.fundedReconcileTitlePast":   "Scale every budget down so %s matches the money that arrived. That period has already ended, so this changes what CashFlux reports about it — previewed budget by budget, and undoable",
	"budgets.fundedReconcileSummary":     "Lowers %s by %s%% for %s. You can make it permanent, and see every budget's before and after, before anything changes.",
	"budgets.fundedReconcileSummaryPast": "Lowers %s by %s%% for %s, a period that has already ended. You can make it permanent, and see every budget's before and after, before anything changes.",

	// The form's opening sentence. A this-period change never touches a limit, so
	// the permanent wording ("every budget's limit") cannot be shared.
	// Why some budgets are left out. Silence here reads as a bug the next time
	// someone counts the rows against "every budget".
	"budgets.adjustAllSkipInvert":         "%s left out: %s. A one-off change already recorded for this period would leave the plan at or below zero at this percentage. Undo that change first, or pick a different one.",
	"budgets.adjustAllSkipEmpty":          "%s left out: %s. There is no amount to take a percentage of yet.",
	"budgets.adjustAllSkipUnknownOverlay": "%s left out: %s. This period's figures for these aren't available, so neither what a change would scale nor what it would leave behind can be stated honestly.",

	"budgets.adjustAllIntroThis": "Raise or lower what every budget has to spend in the period you're looking at, by the same percentage. Each budget's own limit is left as it is. Nothing changes until you apply it.",
}

func init() {
	for k, v := range adjustScopeKeys {
		english[k] = v
	}
}
