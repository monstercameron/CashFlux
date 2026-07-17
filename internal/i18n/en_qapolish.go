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
}

func init() {
	for k, v := range qaPolishKeys {
		english[k] = v
	}
}
