// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// Budget attention filters (UX-05): clicking a count in the budgets issues rail
// narrows the list to exactly the budgets behind that count, so no number on the
// page is unexplainable. "" shows everything.
const (
	BudgetAttentionOver = "over" // only budgets over their limit
	BudgetAttentionNear = "near" // only budgets near their limit
)

// UseBudgetAttention returns the shared atom holding the /budgets attention filter.
// The issues rail's clickable counts set it; the list tile narrows to it and shows a
// dismissible "Showing only…" chip. Ephemeral — a drill-down, not a preference.
func UseBudgetAttention() state.Atom[string] { return state.UseAtom("budgets:attention", "") }

// UseBudgetDensitySeeded returns the /budgets density atom like UseBudgetDensity,
// but when the user has never made an explicit choice it defaults to the compact
// list for households with many budgets (UX-05: past six cards the full-card view
// stops fitting on a screen). An explicit persisted choice always wins.
func UseBudgetDensitySeeded(manyBudgets bool) state.Atom[string] {
	d := kvGet(budgetDensityStoreID)
	if d == "" && manyBudgets {
		d = BudgetDensityCompact
	}
	if d != BudgetDensityCompact {
		d = BudgetDensityComfortable
	}
	return state.UseAtom("budgets:density", d)
}
