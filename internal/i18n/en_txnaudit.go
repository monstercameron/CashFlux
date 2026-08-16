// SPDX-License-Identifier: MIT

package i18n

// txnAuditKeys holds English strings for the Transactions-page audit expansion
// (C560–C572): the ledger period/view bar, the gated bulk categorize, the
// exclude-from-reports confirmation, the row Edit affordance, the receipt-split
// step feedback, the payment-link and split validity states, the scoped
// quick-filter counts, and the grouped row menu. Kept in its own file (not en.go)
// so the change stays additive and off the concurrent WIP file. Registered at init.
var txnAuditKeys = Catalog{
	// --- C560: one period, one view ---------------------------------------------
	// The ledger's period bar. The label is derived from the ledger's own date
	// range, so it can never disagree with the rows beneath it.
	"txnperiod.prev":       "Earlier month",
	"txnperiod.next":       "Later month",
	"txnperiod.thisMonth":  "This month",
	"txnperiod.allDates":   "All dates",
	"txnperiod.allLabel":   "All dates",
	"txnperiod.scopeAria":  "Period shown: %s",
	"txnperiod.viewLabel":  "View",
	"txnperiod.viewLedger": "Ledger",
	"txnperiod.viewCal":    "Calendar",
	// Shown while the ledger is on a hand-picked range rather than a whole month,
	// so the bar states its real scope instead of naming a month it isn't showing.
	"txnperiod.customRange": "%s – %s",
	"txnperiod.oneDay":      "%s",

	// --- C561: Bulk categorize needs a real choice ------------------------------
	"transactions.bulkPickCategory":    "Choose a category",
	"transactions.bulkCatDisabled":     "Pick a category first — then Categorize applies it to the selected transactions.",
	"transactions.bulkClearCat":        "Clear category",
	"transactions.bulkClearCatTitle":   "Remove the category from the selected transactions, leaving them uncategorized.",
	"transactions.bulkClearCatConfirm": "Clear the category on %d? They become uncategorized, so they drop out of every category budget and report until you file them again.",
	"transactions.bulkOpClearedCat":    "Cleared the category on %d",
	"transactions.bulkCatApplied":      "Filed %d under %s",

	// --- C562: excluding from reports is a reporting change, so it asks ---------
	"transactions.excludeConfirm":    "Exclude “%s” from reports? Your account balances do not change — this charge just stops counting toward budgets, spending and reports.",
	"transactions.excludeConfirmBtn": "Exclude from reports",
	"transactions.excludedOne":       "“%s” no longer counts toward budgets or reports",
	"transactions.includedOne":       "“%s” counts toward budgets and reports again",

	// How a transaction refers to itself in a sentence when it has neither a
	// description nor a payee to be named by.
	"transactions.thisTransaction": "this transaction",

	// --- C563: editing a row is a visible, keyboard-reachable action ------------
	"transactions.rowEdit":     "Edit",
	"transactions.rowEditAria": "Edit %s",
	"transactions.rowEditHint": "Open this transaction to edit it",

	// --- C564: the receipt split always says what happens next -----------------
	"receiptsplit.choosing": "Choose a photo of the receipt to read.",
	"receiptsplit.noTxn":    "That transaction is no longer in the ledger, so there is nothing to split.",
	"receiptsplit.noPicker": "This browser would not open a file picker. Attach the receipt from the row's paperclip instead, or split by hand.",

	// --- C565: a payment-link dialog cannot commit a no-op ----------------------
	"txnlink.nothingToSave": "Pick a bill or a subscription to link this payment to.",
	"txnlink.clearHint":     "Saving now removes the links this payment already has.",
	"txnlink.saveRemove":    "Remove link",

	// --- C566: a split is not balanced while a line is unfinished ---------------
	"splitEditor.draftRow":      "Empty line — fill it in or remove it before saving.",
	"splitEditor.incomplete":    "One line still needs a category and an amount",
	"splitEditor.needTwoShort":  "A split needs at least two complete lines",
	"splitEditor.unassigned":    "%s not assigned yet",
	"splitEditor.cannotSaveYet": "Finish every line before saving.",

	// --- C568: quick-filter counts describe what you are looking at ------------
	"transactions.presetCountAria": "%s — %d in the current view",
	"transactions.presetScopeNote": "Counts are for the transactions your current search and filters show.",

	// --- C571: a merge or delete names what survives ---------------------------
	"duplicates.mergeConfirmNamed":  "Merge %d entries into one? “%s” on %s stays — the other %d are removed. You can undo this from Activity.",
	"duplicates.deleteConfirmNamed": "Delete this copy of “%s” (%s, %s)? The entry kept at the top of the group is untouched. You can undo this from Activity.",
	"duplicates.deleteConfirmBtn":   "Delete duplicate",
	"duplicates.mergeConfirmBtn":    "Merge into one",
	"duplicates.survivorLabel":      "Kept: %s",
	"duplicates.resultCount":        "%d entries become 1.",

	// --- C572: the row menu groups by what an action costs you -----------------
	"txnmenu.sectionOrganize": "Organize",
	"txnmenu.sectionLinks":    "Links",
	"txnmenu.sectionReports":  "Reporting",
	"txnmenu.sectionRemove":   "Remove",
}

func init() {
	for k, v := range txnAuditKeys {
		english[k] = v
	}
}
