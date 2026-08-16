// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

// Quick-add kind seeds: which side of the ledger the panel should open on.
const (
	// QuickAddExpense and QuickAddIncome are the two directions the quick-add
	// panel's kind toggle understands. They are the toggle's own option values, so a
	// seed sets the control rather than a parallel piece of state that could drift
	// from it.
	QuickAddExpense = "Expense"
	QuickAddIncome  = "Income"
)

const quickAddKindAtomID = "txn:quickAddKind"

// UseQuickAddKind returns the shared atom holding the direction the quick-add
// panel should open on — "" meaning "whatever the panel defaults to".
//
// C573: the page offered three ways to start a transaction and all three landed on
// the same form in the same state, so none of them told the user what they were
// about to create. A caller that knows the answer ("Income…") can now say so, and
// the entry point's label becomes a promise the destination keeps.
//
// It is consumed once, when the panel opens, and cleared — a seed, not a mode. If
// it persisted, the next plain "+ Add transaction" would silently inherit the last
// choice made from a menu.
func UseQuickAddKind() state.Atom[string] {
	a := state.UseAtom(quickAddKindAtomID, "")
	capturedQuickAddKind = a
	quickAddKindCaptured = true
	return a
}

var (
	capturedQuickAddKind state.Atom[string]
	quickAddKindCaptured bool
)

// OpenQuickAddAs opens the quick-add panel pre-set to a direction, from outside a
// component render (a menu item's callback). Falls back to opening the panel
// unseeded if the atom has not been captured yet, which is the right failure: the
// user still gets the form.
func OpenQuickAddAs(kind string) {
	if quickAddKindCaptured {
		capturedQuickAddKind.Set(kind)
	}
	SetQuickAdd(true)
}

// TakeQuickAddKind reads and clears the seed. Called by the panel as it opens.
func TakeQuickAddKind() string {
	if !quickAddKindCaptured {
		return ""
	}
	v := capturedQuickAddKind.Get()
	if v != "" {
		capturedQuickAddKind.Set("")
	}
	return v
}
