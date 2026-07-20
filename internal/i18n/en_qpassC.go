// SPDX-License-Identifier: MIT

package i18n

// qpassCKeys holds English strings added by the 2026-07-19 v1.2.7 black-box
// review, lane C: the guided, finishable reconcile-to-statement workflow — a
// sticky math header, a standing explanation of what finishing requires, a
// "Mark all cleared" bulk action with an outcome preview, and clearer Finish
// wording. Merged via init so the shared en.go stays untouched by this
// concurrent lane.
var qpassCKeys = Catalog{
	// Standing copy explaining what finishing a reconciliation requires — shown
	// in the flow whenever it isn't yet balanced.
	"accounts.reconExplain": "Clear each transaction that appears on your statement. When the difference reaches zero, you can finish reconciling.",
	// The prominent, always-visible remaining-difference readout in the sticky
	// header. %s = the formatted signed difference (e.g. "+$40.50").
	"accounts.reconRemainingLabel": "Remaining difference: ",
	// The prominent finish action, shown once the difference is zero. Clearer
	// than the older "Record reconciliation" — it names the outcome.
	"accounts.reconFinishAction": "Finish reconciliation",

	// --- "Mark all cleared" bulk action -------------------------------------
	"accounts.reconMarkAll":      "Mark all cleared",
	"accounts.reconMarkAllTitle": "Mark every uncleared transaction on this account as cleared",
	// Outcome preview shown beside the bulk button once a statement balance is
	// typed. %d = how many transactions would flip to cleared.
	"accounts.reconMarkAllPreviewMatch": "Clearing all %d will match the statement exactly.",
	// %d = count, %s = the formatted difference that would remain afterward.
	"accounts.reconMarkAllPreviewGap": "Clearing all %d still leaves a %s difference.",
	// Undoable toast summary after the bulk clear lands. %d = how many cleared.
	"accounts.reconMarkAllDone": "%d transactions marked cleared",
}

func init() {
	for k, v := range qpassCKeys {
		english[k] = v
	}
}
