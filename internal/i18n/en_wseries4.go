// SPDX-License-Identifier: MIT

package i18n

// wSeries4Keys holds English strings for the W6 budgets forward-planning lane
// (C371 discoverable year planner, C393 income scenario mode, C394 future-month
// projection) on the annual grid. Kept in its own file so this lane's copy never
// collides with the concurrent-WIP catalog; registered at init.
var wSeries4Keys = Catalog{
	// C371 — "Plan the year" entry point.
	"budgets.planYearTitle":    "Plan the year",
	"budgets.planYearHint":     "See the whole year and plan ahead",
	"budgets.planYearShowAria": "Show the year planner",
	"budgets.planYearHideAria": "Hide the year planner",

	// C394 — projection legend + cell explanation.
	"budgets.gridLegendActual":    "Actual",
	"budgets.gridLegendPlanned":   "Planned",
	"budgets.gridLegendProjected": "Projected",
	"budgets.gridProjectedTitle":  "Projected %s from recurring bills and goal plans",

	// C393 — income scenario mode.
	"budgets.scenarioToggle":      "Scenario",
	"budgets.scenarioToggleAria":  "Try an income scenario",
	"budgets.scenarioHint":        "See what goes underfunded if your income changes — nothing is saved.",
	"budgets.scenarioLabel":       "If income changes by",
	"budgets.scenarioLess":        "Lower income by $100 a month",
	"budgets.scenarioMore":        "Raise income by $100 a month",
	"budgets.scenarioReset":       "Reset the scenario",
	"budgets.scenarioAllFunded":   "Everything still funded",
	"budgets.scenarioUnderfunded": "%d underfunded",
	"budgets.scenarioUnderTitle":  "Underfunded by %s in this scenario",
	"budgets.scenarioLegend":      "Underfunded",
}

func init() {
	for k, v := range wSeries4Keys {
		english[k] = v
	}
}
