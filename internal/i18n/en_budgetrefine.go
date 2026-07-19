// SPDX-License-Identifier: MIT

package i18n

// budgetRefineKeys holds English strings added by the 2026-07-19 Budgets UX
// refinement pass: naming the single top review queue, and folding the page's
// view/tool controls into one "Budget settings" popover. Merged via init so the
// shared en.go is never touched by this concurrent lane.
var budgetRefineKeys = Catalog{
	// --- The single authoritative review queue (top "Needs attention" strip) ---
	// The subtitle now NAMES what the flagged budgets are, instead of the vaguer
	// "N of your budgets need a look". %d = number of budgets in the queue.
	"budgetrefine.queueNamed":    "%d budgets over or near their limit",
	"budgetrefine.queueNamedOne": "1 budget over or near its limit",

	// --- One control bar: the "Budget settings" popover ---
	// Trigger + section headers inside the popover that now holds the method
	// picker, sort order, compact-list toggle, and the bulk tools.
	"budgetrefine.settings":      "Budget settings",
	"budgetrefine.settingsTitle": "Budgeting method, sort order, layout, and bulk tools",
	"budgetrefine.sectionView":   "View",
	"budgetrefine.sectionTools":  "Bulk tools",
}

func init() {
	for k, v := range budgetRefineKeys {
		english[k] = v
	}
}
