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
}

func init() {
	for k, v := range changesetKeys {
		english[k] = v
	}
}
