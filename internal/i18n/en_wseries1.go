// SPDX-License-Identifier: MIT

package i18n

// wseries1Keys holds English strings added by the W1 surfacing lane:
//   - C363: a first-class Rules entry on the Transactions toolbar + a per-row
//     "create rule from this transaction" affordance.
//   - C364: the undo story at the moment of risk — every bulk-mutation
//     completion toast names the reversal path (Undo / Activity).
//
// Merged via init so this file never touches the shared en.go.
var wseries1Keys = Catalog{
	// C363 — surfacing Rules from Transactions.
	// %d = number of active auto-categorization rules.
	"transactions.rulesButton": "Rules (%d)",
	"transactions.createRule":  "Create rule from this transaction",

	// C364 — the undo story. %s = the per-op summary (e.g. "12 transactions
	// recategorized"). The trailing clause is the same everywhere so the reversal
	// path reads consistently: keyboard undo, or the full history in Activity.
	"toast.undoStory":    "%s · Undo (Ctrl+Z) · View in Activity",
	"activity.viewLink":  "View in Activity",
	"activity.viewTitle": "Open the Activity timeline to review or undo this change",

	// Undoable import summaries posted as toasts at commit time.
	// %s = "N transactions".
	"documents.importUndo":        "Imported %s",
	"documents.receiptImportUndo": "Imported the receipt as one transaction",
}

func init() {
	for k, v := range wseries1Keys {
		english[k] = v
	}
}
