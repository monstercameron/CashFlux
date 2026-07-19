// SPDX-License-Identifier: MIT

package i18n

// dashFocusKeys explains the dashboard "Focus" view picker (C366). A reviewer
// couldn't tell whether a Focus view changed the layout, the content, or only
// emphasis — it actually swaps the widget set AND compacts the hero. These add a
// one-line description per preset plus a standing subtitle that says so. Merged
// via init so this file never touches en.go.
var dashFocusKeys = Catalog{
	// Standing subtitle under the picker: what a Focus view does, in one line.
	"dashboard.presetSubtitle": "Each view swaps the tiles shown and how much detail you see.",

	// One-line description per preset, shown for the current selection.
	"dashboard.presetDescDefault":  "Everything — your full dashboard.",
	"dashboard.presetDescDaily":    "Today's essentials: balances, bills due, and quick actions.",
	"dashboard.presetDescPayday":   "Fresh paycheck: income, budgets, and what to set aside.",
	"dashboard.presetDescMonthEnd": "Wrap the month: spending recap, budgets, and net change.",
	"dashboard.presetDescDebt":     "Pay down debt: balances owed and payoff progress.",
	"dashboard.presetDescGoals":    "Your goals: progress, pace, and what's next.",
}

func init() {
	for k, v := range dashFocusKeys {
		english[k] = v
	}
}
