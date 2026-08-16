// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

// BudgetDrill records which budget a filtered ledger was opened from, so the
// Transactions page can say so (C585).
//
// A budget's drill-through applies several category filters at once — the
// tracked categories AND their descendants — which is correct but arrives as an
// anonymous pile of chips. Naming the origin turns that pile into a sentence the
// user can check against the card they clicked, and gives them one control that
// undoes the whole scope rather than three.
//
// A zero value means the ledger was not opened from a budget.
type BudgetDrill struct {
	// BudgetID identifies the origin budget; empty means "no budget origin".
	BudgetID string
	// Name is the budget's display title, as shown on its card.
	Name string
	// Spent is the formatted spend the card was showing when it was clicked, so
	// an empty ledger can be reconciled against the figure that led here.
	Spent string
	// Descendants is true when the scope reaches past the budget's own tracked
	// categories into their sub-categories — the case worth naming out loud.
	Descendants bool
	// Tags is true when the budget also counts charges by tag, so the note can
	// say that tagged charges are included whatever their category.
	Tags bool
	// Scope is the category/tag selection the drill applied, remembered so the
	// note can retire itself the moment the ledger stops showing that scope. A
	// label that outlives what it describes is worse than no label.
	Scope BudgetDrillScope
}

// BudgetDrillScope is the identifying part of the filter a budget drill applied.
type BudgetDrillScope struct {
	Category   string
	Categories string
	Tags       string
}

// budgetDrill is a plain package variable, not an atom, deliberately.
//
// It is written on the /budgets page and read on the first render of
// /transactions — a surface that has not mounted yet at write time, so an atom's
// captured reference would still be nil and the write would silently do nothing.
// Navigation re-renders the target anyway, which is all the reactivity this
// needs.
var budgetDrill BudgetDrill

// SetBudgetDrill records the budget a ledger view was opened from.
func SetBudgetDrill(d BudgetDrill) { budgetDrill = d }

// ClearBudgetDrill forgets the origin — called when the user drops the scope the
// drill applied, so the note never outlives what it describes.
func ClearBudgetDrill() { budgetDrill = BudgetDrill{} }

// BudgetDrillFor returns the recorded drill origin, but only while the given
// filter still carries the scope that drill applied. Any other filter — the user
// cleared it, picked a different category, or arrived from somewhere else — gets
// the zero value, so the ledger never claims to be showing a budget it is not.
func BudgetDrillFor(f TxFilter) BudgetDrill {
	if budgetDrill.BudgetID == "" {
		return BudgetDrill{}
	}
	if (BudgetDrillScope{Category: f.Category, Categories: f.Categories, Tags: f.Tags}) != budgetDrill.Scope {
		return BudgetDrill{}
	}
	return budgetDrill
}
