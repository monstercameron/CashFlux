// SPDX-License-Identifier: MIT

package i18n

// lane2Keys holds English strings added by the 2026-07-17 dashboard-defaults
// lane (#76: Daily check-in recommendation, edit-layout mode, bills glance cap,
// money-vs-household attention grouping). Merged via init so this file never
// touches the shared en.go.
var lane2Keys = Catalog{
	// #76 — edit-layout mode toggle.
	"dashboard.editLayout":      "Edit layout",
	"dashboard.editLayoutDone":  "Done",
	"dashboard.editLayoutTitle": "Rearrange or resize the dashboard tiles",

	// #76 — one-time Daily check-in recommendation.
	"dashboard.dailyNudgeText":      "You've settled in — want a calmer daily view? Daily check-in shows just today's essentials.",
	"dashboard.dailyNudgeUse":       "Try Daily check-in",
	"dashboard.dailyNudgeUseTitle":  "Switch the dashboard to the Daily check-in view (Everything stays one click away)",
	"dashboard.dailyNudgeKeep":      "Keep Everything",
	"dashboard.dailyNudgeKeepTitle": "Keep the full dashboard and don't ask again",

	// #76 — Upcoming bills glance cap.
	"dashboard.viewAllBills": "View all %d bills",

	// #76 — Needs attention grouping.
	"dashboard.attnMoney":     "Money",
	"dashboard.attnHousehold": "Household",

	// #62 — "Continue where you left off" resume card.
	"dashboard.resumeTitle":       "Continue where you left off",
	"dashboard.resumeDismiss":     "Hide for now",
	"dashboard.resumeReconcile":   "Reconciliation of %s is half-done.",
	"dashboard.resumeImport":      "An import review is in progress — %d rows are waiting.",
	"dashboard.resumeImportStale": "An import didn't finish, and its rows couldn't be kept. Start it again from Documents.",
	"dashboard.resumeReview":      "%d transactions are waiting in the Review inbox.",
	"dashboard.resumeOverassign":  "Your budgets assign %s more than this period brings in.",
	"dashboard.resumeContinue":    "Continue",
	"dashboard.resumeResume":      "Resume",
	"dashboard.resumeResolve":     "Resolve",
	"dashboard.resumeOpenDocs":    "Open Documents",
}

func init() {
	for k, v := range lane2Keys {
		english[k] = v
	}
}
