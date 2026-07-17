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
}

func init() {
	for k, v := range lane4Keys {
		english[k] = v
	}
}
