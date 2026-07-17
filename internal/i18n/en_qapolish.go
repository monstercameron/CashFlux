// SPDX-License-Identifier: MIT

package i18n

// qaPolishKeys holds strings added by the 2026-07-17 QA remediation polish
// (L2 saved-state feedback, L5 goal monthly-pace labeling). Merged via init so
// this file never touches the shared en.go.
var qaPolishKeys = Catalog{
	// L2 — auto-saving settings announce their save instead of persisting silently.
	"settings.methodSaved": "Budgeting method saved.",
	"settings.baseSaved":   "Base currency saved.",
	"common.savedToast":    "Saved.",

	// L5 — the goal card's calculated pace is labeled as the REQUIRED pace, and a
	// user-set monthly plan is shown as its own figure so the two never blur.
	"goals.figMonthlyNeeded": "Needed / mo",
	"goals.figMonthlyPlan":   "Your plan / mo",

	// M7 — quick-add "More details" fold: the metadata that used to require
	// save-then-reopen, available at creation.
	"quickAdd.moreDetails": "More details (tags, member, note…)",
	"quickAdd.tags":        "Tags",
	"quickAdd.tagsPh":      "vacation, reimbursable…",
	"quickAdd.member":      "Member",
	"quickAdd.memberAuto":  "From the account (default)",
	"quickAdd.note":        "Note",
	"quickAdd.notePh":      "A memo for this one transaction",
	"quickAdd.cleared":     "Cleared (settled at the bank)",
	"quickAdd.exclude":     "Exclude from reports and spending totals",
}

func init() {
	for k, v := range qaPolishKeys {
		english[k] = v
	}
}
