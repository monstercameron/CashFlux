// SPDX-License-Identifier: MIT

package i18n

// lane4Keys holds English strings added by the 2026-07-17 lane-4 remediation
// batch (#48 reconciliation resolution toolkit, #77 mark-all preview/undo,
// #55 checkpoints, #57 import dependability, #63 transaction trust, #58
// receipt matching, #53 data health). Merged via init so the shared en.go is
// never touched by this concurrent lane.
var lane4Keys = Catalog{
	// #48 — reconciliation discrepancy resolution (QA R3 CF-02).
	// %s = the statement closing date the user typed.
	"accounts.reconAdjustDesc": "Reconciliation adjustment (statement %s)",
	// %s = the formatted adjustment amount.
	"accounts.reconAdjustPosted": "Posted a %s adjustment to match the statement.",
	"accounts.reconAdjustAction": "Post adjustment",
	// %s = the formatted unresolved difference.
	"accounts.reconForceConfirm":  "Finish reconciling with a %s difference still unresolved? The gap will be recorded in this account's history.",
	"accounts.reconForceRecorded": "Reconciliation recorded with a %s unresolved difference.",
	"accounts.reconForceAction":   "Finish with difference",
	"accounts.reconDraftSaved":    "Saved — this reconciliation will pick up where you left off.",
	"accounts.reconSaveDraft":     "Save & finish later",
	// %s = the reopened statement's date.
	"accounts.reconReopened":     "Reopened the %s reconciliation — its figures are back in the form.",
	"accounts.reconReopenAction": "Reopen last",
	"accounts.reconInvestigate":  "Investigate cleared",
	"accounts.reconResolveHint":  "Doesn't match yet? Post an adjustment for the difference, dig into the cleared transactions, or finish with the gap noted.",
	// %s = the formatted difference recorded on a forced reconciliation.
	"accounts.reconForcedTag": "off by %s",
	"accounts.reconDraftNote": "Resumed from your saved draft.",

	// #77 — Mark-all-updated preview + undo. First %s = "N balances", second
	// %s = the (possibly truncated) account-name list.
	"accounts.markAllConfirm": "Mark %s as confirmed just now? This updates: %s. You can undo it afterwards.",
	// %s = the leading names, %d = how many more beyond them.
	"accounts.markAllMore": "%s and %d more",

	// #55 — pre-operation safety checkpoints (Settings → Data).
	"ckpt.section":     "Safety checkpoints",
	"ckpt.sectionHint": "Taken automatically before imports and bulk changes — restore one to roll everything back to just before that operation.",
	"ckpt.empty":       "No checkpoints yet. One is saved automatically before each import or bulk change.",
	// %d = blob size in KB.
	"ckpt.sizeKB":       "%d KB",
	"ckpt.restoreBtn":   "Restore",
	"ckpt.restoreTitle": "Replace the current data with this checkpoint",
	// %s = the checkpoint's label.
	"ckpt.restoreConfirm": "Restore \"%s\"? Everything changed since that checkpoint will be lost.",
	"ckpt.restored":       "Restored \"%s\" — your data is back to just before that operation.",
	"ckpt.restoreErr":     "Couldn't restore that checkpoint.",
	// %s = the checkpoint's label.
	"ckpt.deleteAria": "Delete checkpoint %s",
	// Labels stamped on the checkpoints themselves. %s = a "N things" phrase.
	"ckpt.beforeApplyRules":    "Before applying rules to %s",
	"ckpt.beforeBulkDelete":    "Before deleting %s",
	"ckpt.beforeBulkRecat":     "Before recategorizing %s",
	"ckpt.beforeCoverAll":      "Before covering overspending across %s",
	"ckpt.beforeAllocation":    "Before applying %s from the allocation",
	"ckpt.beforeCSVImport":     "Before a CSV import",
	"ckpt.beforeDocImport":     "Before importing %s from a document",
	"ckpt.beforeReceiptImport": "Before importing a receipt",

	// #57 — import dependability: preflight preview, why-matched duplicates,
	// transfer-pair detection, richer summaries, per-run roll-back.
	// %s = "N new transactions", %d = duplicate count.
	"documents.preflightCounts": "Ready to import %s. %d duplicates will be skipped.",
	// %s ×4 = account name, balance before, balance after, signed net.
	"documents.preflightBalance": "%s: %s → %s (%s).",
	"documents.preflightJumpWarn": "This is a big jump for this account — double-check the file (and its sign convention) before importing.",
	// %d = duplicate count.
	"documents.preflightWhyDups":  "Why are %d rows duplicates?",
	"documents.preflightDupLedger": "already in your ledger (same date, amount, and description)",
	"documents.preflightDupBatch":  "repeated within this file",
	// %d = how many more beyond the shown sample.
	"documents.preflightMoreDups": "…and %d more.",
	// %d = detected pair count.
	"documents.preflightPairs": "%d rows look like transfers you already recorded in another account — importing them may double-count the move:",
	// %s ×3 = incoming desc, amount, matching existing desc.
	"documents.preflightPairLine": "\"%s\" (%s) mirrors \"%s\"",
	// %d = new-row count.
	"documents.preflightImportNow": "Import %d rows",
	// %s ×2 = balance before, balance after.
	"documents.balanceMoved": "Balance moved %s → %s.",
	"documents.historyHeading": "Recent imports",
	"documents.historyHint":    "Each run's full result — Roll back restores your data to just before that import.",
	"documents.historyKindCSV": "CSV import",
	"documents.historyKindDoc": "Document import",
	// %s = "N transactions".
	"documents.historyImported": "%s imported",
	// %d = skipped count.
	"documents.historySkipped":  "· %d skipped",
	"documents.rollbackBtn":     "Roll back",
	"documents.rollbackTitle":   "Restore your data to just before this import",
	"documents.rollbackConfirm": "Roll back this import? Your data returns to the moment before it ran — the imported rows and anything you changed since will be lost.",
	"documents.rolledBack":      "Rolled back — your data is back to just before that import.",

	// #63 — transaction trust details.
	// %s ×2 = account name, its new balance.
	"transactions.savedImpact":          "Saved — %s is now %s.",
	"transactions.reconciledBadgeTitle": "Reconciled — vouched for by a recorded statement reconciliation.",
	// %s ×3 = from-account, amount, to-account.
	"accounts.xferSemanticsAsset":     "Moves %s down by %s and %s up by the same — your total money doesn't change, and neither side counts as spending.",
	"accounts.xferSemanticsLiability": "Takes %s down by %s and reduces what you owe on %s by the same — a debt payment, not spending.",
	"txnhistory.title":                "Transaction history",
	"txnhistory.menuAction":           "History",
	"txnhistory.empty":                "No recorded changes for this transaction yet. Edits, rule applications, and imports made from now on are tracked here.",
	"txnhistory.scopeNote":            "Every change this device recorded for this transaction, newest first.",
	"txnhistory.actorYou":             "You",
	// %d = estimated minutes remaining at the session's pace.
	"review.paceEstimate": "≈ %d min left at this pace",

	// #58 — receipt-to-transaction matching.
	// %d = candidate count.
	"documents.receiptMatchLead": "This receipt looks like %d charges already in your ledger — attach it instead of creating a duplicate:",
	"documents.receiptMatchHint": "Attaching puts the receipt's category breakdown on the existing charge. \"Import\" below still creates a new transaction.",
	"documents.receiptMatchSameDay": "same day",
	// %d = days between receipt and charge.
	"documents.receiptMatchDaysApart": "%d days apart",
	"documents.receiptMatchMerchant":  "merchant matches",
	"documents.receiptAttachBtn":      "Attach to this charge",
	"documents.receiptAttachTitle":    "Put this receipt's breakdown on the existing transaction — no new row",
	// %s ×2 = the charge's description, "N categories".
	"documents.receiptAttached": "Attached the receipt to \"%s\" — split across %s, no new transaction created.",
}

func init() {
	for k, v := range lane4Keys {
		english[k] = v
	}
}
