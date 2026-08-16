// SPDX-License-Identifier: MIT

package i18n

// changesetKeys holds English copy for the AG1 changeset review card and the
// AG20 session receipt. Kept in its own file (not en.go) so it doesn't touch the
// concurrent-WIP catalog.
var changesetKeys = Catalog{
	// --- AG1 review card ---
	"changeset.title":       "Proposed changes",
	"changeset.subtitle":    "Review each step, then apply the ones you want.",
	"changeset.itemAria":    "Include this step",
	"changeset.applyAll":    "Apply all (%d)",
	"changeset.applyNone":   "Nothing selected",
	"changeset.dismiss":     "Not now",
	"changeset.dismissAria": "Dismiss these proposed changes",
	// --- receipt after apply ---
	"changeset.receiptTitle": "Applied %d of %d",
	"changeset.receiptOne":   "Applied 1 change",
	// %s = the failing step, %s = the error.
	"changeset.failed":      "Stopped at “%s”: %s. Earlier steps were applied.",
	"changeset.undoAll":     "Undo all",
	"changeset.undoAllAria": "Undo every change the assistant just applied",
	"changeset.undone":      "Undid the assistant's changes.",
	"changeset.applied":     "Applied %d change(s).",
	// --- AG20 cumulative session receipt ---
	"changeset.sessionAria": "What the assistant did in this chat",
	// --- C389 per-action history ---
	// %d = how many individual changes the assistant has made this session.
	"changeset.historySummary": "Every change, one by one (%d)",
	"changeset.undoLast":       "Undo the last one",
	"changeset.openActivity":   "See them in Activity",
	// --- C390 per-conversation model + token cap ---
	"assistant.budgetLabel": "Cap",
	"assistant.budgetPick":  "Cap what this chat may spend in total",
	"assistant.budgetNone":  "No cap",
	// %s = a token figure, already formatted with thousands separators.
	"assistant.budgetOption": "%s tokens",
	"assistant.budgetUsed":   "%s used",
	"assistant.budgetLeft":   "%s left",
	// %s = the cap that was reached. It names the way out, because the cap is the
	// user's own and changing their mind is a legitimate answer.
	"insights.budgetSpent": "This chat has spent its %s-token cap. Raise the cap above, or start a new chat.",
	// --- C391 confidence on inferred findings ---
	"assistant.confidenceHint": "This one was worked out from a pattern, not read straight off your ledger — worth checking before you act on it.",
}

func init() {
	for k, v := range changesetKeys {
		english[k] = v
	}
}
