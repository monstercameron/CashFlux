// SPDX-License-Identifier: MIT

package dashlayout

// Dashboard presentation presets: curated widget sets for a moment — checking
// in daily, planning a payday, closing a month, attacking debt, or funding
// goals. Applying one replaces the layout items (the user can still drag/resize
// afterwards; Reset in Settings restores the default). Every id must exist in
// the DefaultItems catalog — enforced by the preset test.

// PresetKeys lists the available presets in display order.
var PresetKeys = []string{"daily", "payday", "monthend", "debt", "goals"}

// presetItems maps each preset to its curated widget list. Spans echo the
// catalog sizes; Pack backfills the grid, so lists don't have to tile exactly.
var presetItems = map[string][]Item{
	// Daily check-in: what needs me, can I spend, what's coming, what happened.
	"daily": {
		{ID: "attention", ColSpan: 4, RowSpan: 1},
		{ID: "kpi-safetospend", ColSpan: 2, RowSpan: 1},
		{ID: "forecast", ColSpan: 2, RowSpan: 1},
		{ID: "bills", ColSpan: 2, RowSpan: 1},
		{ID: "recent", ColSpan: 2, RowSpan: 2},
		{ID: "todo", ColSpan: 2, RowSpan: 1},
	},
	// Payday: fund what's due before the next check, then the plans.
	"payday": {
		{ID: "attention", ColSpan: 4, RowSpan: 1},
		{ID: "kpi-safetospend", ColSpan: 2, RowSpan: 1},
		{ID: "forecast", ColSpan: 2, RowSpan: 1},
		{ID: "bills", ColSpan: 2, RowSpan: 1},
		{ID: "budgets", ColSpan: 2, RowSpan: 2},
		{ID: "goals", ColSpan: 2, RowSpan: 1},
		{ID: "accounts", ColSpan: 2, RowSpan: 1},
	},
	// Month end: how the month went and how the plan held up.
	"monthend": {
		{ID: "monthly-recap", ColSpan: 4, RowSpan: 1},
		{ID: "breakdown", ColSpan: 2, RowSpan: 1},
		{ID: "cashflow", ColSpan: 2, RowSpan: 1},
		{ID: "budgets", ColSpan: 2, RowSpan: 2},
		{ID: "trend", ColSpan: 2, RowSpan: 2},
		{ID: "health", ColSpan: 2, RowSpan: 1},
	},
	// Debt focus: what's owed, what it costs, and the runway to pay it down.
	"debt": {
		{ID: "attention", ColSpan: 4, RowSpan: 1},
		{ID: "kpi-liabilities", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-safetospend", ColSpan: 1, RowSpan: 1},
		{ID: "forecast", ColSpan: 2, RowSpan: 1},
		{ID: "accounts", ColSpan: 2, RowSpan: 2},
		{ID: "cashflow", ColSpan: 2, RowSpan: 1},
		{ID: "trend", ColSpan: 2, RowSpan: 1},
	},
	// Goals focus: progress, states, and the money available to push them.
	"goals": {
		{ID: "goal-states", ColSpan: 4, RowSpan: 1},
		{ID: "goals", ColSpan: 2, RowSpan: 1},
		{ID: "kpi-assets", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-safetospend", ColSpan: 1, RowSpan: 1},
		{ID: "forecast", ColSpan: 2, RowSpan: 1},
		{ID: "todo", ColSpan: 2, RowSpan: 1},
		{ID: "trend", ColSpan: 2, RowSpan: 1},
	},
}

// PresetItems returns a copy of the preset's widget list, and whether the key
// names a preset.
func PresetItems(key string) ([]Item, bool) {
	items, ok := presetItems[key]
	if !ok {
		return nil, false
	}
	out := make([]Item, len(items))
	copy(out, items)
	return out, true
}
