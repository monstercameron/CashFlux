// SPDX-License-Identifier: MIT

package i18n

// lane5Keys holds English strings added by the 2026-07-17 goal/budget/household
// refinement lane (#51 slider accessibility, #70 budgets historical wording,
// #71 compact goal cards, #64 month close, #65 goals refinement, #66 household
// clarity). Merged via init so this file never touches the shared en.go.
var lane5Keys = Catalog{
	// #51 — the contribution planner's direct numeric entry.
	"goals.planAmountLabel": "Monthly contribution amount",

	// #70 (UX-05) — budgets: historical-period wording, explainable counts, Automate menu.
	"budgets.histSpendCap":      "%s spending",
	"budgets.histUnspent":       "Unspent",
	"budgets.histIssuesRail":    "%d items to review from this period",
	"budgets.histIssuesRailOne": "1 item to review from this period",
	"budgets.histOverBanner":    "%d categories ended over budget by %s total.",
	"budgets.histOverBannerOne": "1 category ended over budget by %s.",
	"budgets.histNearBadge":     "%d finished near the limit",
	"budgets.followUpsCount":    "%d unresolved follow-ups",
	"budgets.followUpsCountOne": "1 unresolved follow-up",
	"budgets.followUpsRowBody":  "To-dos linked to these budgets that are still open.",
	"budgets.followUpsRowView":  "View to-dos",
	"budgets.filterShow":        "Show in list",
	"budgets.filterOverTitle":   "Filter the list to the over-budget categories",
	"budgets.filterNearTitle":   "Filter the list to the categories near their limit",
	"budgets.attentionOver":     "Showing over-budget categories only",
	"budgets.attentionNear":     "Showing near-limit categories only",
	"budgets.attentionClear":    "Show all",
	"budgets.automate":          "Automate",
	"budgets.automateTitle":     "Bulk budget tools — last month's spend, auto budget, sweep leftovers, adjust all",
	"budgets.followUpsShow":     "Show the follow-ups",
	"budgets.followUpsHide":     "Hide the follow-ups",

	// #71 (UX-06) — the compact goal card's expand/collapse control.
	"goals.expand":        "Details",
	"goals.expandTitle":   "Show everything on this goal",
	"goals.collapse":      "Less",
	"goals.collapseTitle": "Back to the compact card",
}

func init() {
	for k, v := range lane5Keys {
		english[k] = v
	}
}
