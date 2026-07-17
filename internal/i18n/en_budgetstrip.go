// SPDX-License-Identifier: MIT

package i18n

// budgetStripKeys are the strings for the /budgets three-column status strip and
// the collapsed "issues need attention" rail (2026-07-17 visual audit P0 — keep
// the first budget category above the fold). Kept separate from en.go
// (concurrent WIP) like the other feature key files.
var budgetStripKeys = Catalog{
	// The single collapsed rail that replaces the stacked warning banners.
	"budgets.issuesRail":    "%d issues need attention",
	"budgets.issuesRailOne": "1 issue needs attention",
	"budgets.issuesShow":    "Show details",
	"budgets.issuesHide":    "Hide details",
	// Over-assignment issue row (zero-based: assigned > income pool; simple:
	// budgeted > income) and its resolution affordances.
	"budgets.issueOverAssigned":     "Budgets are over-assigned by %s",
	"budgets.issueOverAssignedBody": "More is assigned than the income you're budgeting with. Lower some category assignments, or budget with more income.",
	"budgets.issueReviewAlloc":      "Review allocations",
	"budgets.issueReviewAllocTitle": "Open the Allocate tool to rebalance what each category gets",
	"budgets.resolveAmount":         "Resolve %s",
	// Column captions for the status strip.
	"budgets.stripSpending": "Spending",
	"budgets.stripPlan":     "Plan",
}

func init() {
	for k, v := range budgetStripKeys {
		english[k] = v
	}
}
