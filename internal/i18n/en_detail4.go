// SPDX-License-Identifier: MIT

package i18n

// detail4Keys holds English strings for the 2026-07-19 fine-detail polish lane 4:
//   - the Goals "Needs a plan" urgency grades (Far behind / Slipping / Watch),
//   - the Goals Compare one-sentence verdict,
//   - the To-do notes click-to-expand affordance, and
//   - the To-do "Templates & tools" rename plus the new Quarterly account review
//     checklist template.
//
// Merged via init so the shared en.go (and other lanes' i18n files) are never touched.
var detail4Keys = Catalog{
	// --- Goals: "Needs a plan" urgency grades -------------------------------------
	// A cluster of stretch goals used to all read "Watch"; these grade how far behind
	// each is (from required-monthly vs. its fair share of free cash) so the group is
	// prioritised. Far behind = danger, Slipping = warn, Watch = neutral.
	"goals.urgencyFarBehind": "Far behind",
	"goals.urgencySlipping":  "Slipping",
	"goals.urgencyWatch":     "Watch",

	// --- Goals: Compare verdict ---------------------------------------------------
	// One honest sentence above the comparison table, derived only from the figures the
	// table already shows (projected landings + monthly plans).
	"goalcompare.vMonthsOne":          "1 month",
	"goalcompare.vMonths":             "%d months",
	"goalcompare.vLeadSoonerCostMore": "At current plans, %s finishes %s earlier than %s, but needs %s more each month.",
	"goalcompare.vLeadSoonerCostLess": "At current plans, %s finishes %s earlier than %s, and needs %s less each month.",
	"goalcompare.vLeadSoonerOnly":     "At current plans, %s finishes %s earlier than %s.",
	"goalcompare.vLeadSameCost":       "At current plans, %s and %s finish around the same time, but %s needs %s more each month.",
	"goalcompare.vLeadSameOnly":       "At current plans, %s and %s finish around the same time.",
	"goalcompare.vLeadCostOnly":       "At current plans, %s needs %s more each month than %s.",

	// --- To-do: note click-to-expand ----------------------------------------------
	// A long task note now clamps to two lines and expands on click (was a single-line
	// ellipsis). These label the toggle for screen-reader and hover.
	"todo.noteToggleExpand":   "Show the full note",
	"todo.noteToggleCollapse": "Collapse the note",

	// --- To-do: Templates & tools + Quarterly review template ---------------------
	// The overflow control renamed from "More tools" to say what it holds. Plus a third
	// checklist template alongside the month-end close and tax prep.
	"todo.templatesTools":       "Templates & tools",
	"todo.checklistQuarterly":   "Quarterly account review",
	"todo.tmplQuarterly":        "Quarterly review — %s",
	"todo.tmplQtrBalances":      "Update every account balance to today",
	"todo.tmplQtrSubscriptions": "Review recurring subscriptions and cancel unused ones",
	"todo.tmplQtrBudgets":       "Check each budget against the last three months",
	"todo.tmplQtrGoals":         "Rebalance goal contributions for the quarter",
}

func init() {
	for k, v := range detail4Keys {
		english[k] = v
	}
}
