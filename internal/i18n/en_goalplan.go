// SPDX-License-Identifier: MIT

package i18n

// goalPlanKeys holds English strings for Agent B's goal features: GL3
// (emergency-fund auto-sizing), GL4 (contribution slider with live ETA), and GL5
// (shared goals with per-member pledges). Kept in its own file (not en.go) so it
// doesn't collide with the concurrently edited catalog. Registered at init.
var goalPlanKeys = Catalog{
	// GL3 — emergency-fund auto-sizing.
	"goals.essentialTitle": "Size your emergency fund",
	// %s = essential monthly amount.
	"goals.essentialMonth": "Your essential month is about %s — fixed commitments plus essential spending.",
	// %s = 3-month target amount.
	"goals.essentialThree": "3-month fund: %s",
	// %s = 6-month target amount.
	"goals.essentialSix":      "6-month fund: %s",
	"goals.essentialSet3":     "Set 3-month target",
	"goals.essentialSet6":     "Set 6-month target",
	"goals.essentialHint":     "Based on your recent essentials. You can change it anytime.",
	"goals.essentialSetToast": "Emergency-fund target updated",

	// GL4 — contribution slider with live ETA.
	"goals.planTitle": "Plan your contribution",
	"goals.planPerMo": "%s/mo",
	// %s = projected finish date, e.g. "Aug 2027".
	"goals.planFinish":      "finishes %s",
	"goals.planNoFinish":    "pick an amount to see a finish date",
	"goals.planOnTrack":     "on track for your target date",
	"goals.planUseThis":     "Use this plan",
	"goals.planSavedToast":  "Monthly contribution set to %s",
	"goals.planSliderLabel": "Monthly contribution",
	// Reverse bridge to budgets.
	"goals.planWhereFrom": "Where does this come from?",

	// GL5 — shared goals with per-member pledges.
	"goals.pledgeTitle": "Shared pledges",
	// %s = member name, %s = pledged monthly amount.
	"goals.pledgeRow": "%s pledged %s/mo",
	// Blame-free standing readouts. %s = member name.
	"goals.pledgeAhead":  "%s is ahead of pledge",
	"goals.pledgeOnPace": "%s is on pace",
	"goals.pledgeBehind": "%s has a little to catch up",
	// %s = member name, %d = whole months ahead.
	"goals.pledgeAheadMonths": "%s is %d months ahead of pledge",
	"goals.pledgeUnassigned":  "Unassigned",
	"goals.pledgeSplitLabel":  "Contributions by member",
}

func init() {
	for k, v := range goalPlanKeys {
		english[k] = v
	}
}
