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
	"goals.figMonthlyNeeded": "Needed / month",
	"goals.figMonthlyPlan":   "Your plan / month",

	// CF-30 — honest chart alt text: each sparkline says what IT shows (the
	// report's net-worth shape used to announce "Recent charges at this merchant").
	"rpta.nwSparkAlt":  "Net worth over the last 12 months",
	"rpta.catSparkAlt": "Monthly spending over the year",

	// M7 — quick-add "More details" fold: the metadata that used to require
	// save-then-reopen, available at creation.
	"quickAdd.moreDetails": "More details (tags, member, note…)",
	"quickAdd.tags":        "Tags",
	"quickAdd.tagsPh":      "e.g. groceries, shared",
	"quickAdd.member":      "Household member",
	"quickAdd.memberAuto":  "From the account (default)",
	"quickAdd.note":        "Note",
	"quickAdd.notePh":      "Add a note for this transaction",
	"quickAdd.cleared":     "Cleared",
	"quickAdd.exclude":     "Exclude from budgets and reports",
}

func init() {
	for k, v := range qaPolishKeys {
		english[k] = v
	}
}
