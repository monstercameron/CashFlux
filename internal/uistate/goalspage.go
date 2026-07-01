// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

// goalEditAtomID keys the shared goal-editor modal selector.
const goalEditAtomID = "goals:edit"

// Goal-editor modes (which form the shell-root flip modal shows).
const (
	GoalEditModeEdit       = "edit"       // full edit form (name/target/date/owner/linked account)
	GoalEditModeContribute = "contribute" // add an amount toward the goal (optionally posting to the ledger)
)

// GoalEdit selects the goal + editor a modal should show. A zero value (empty ID) means
// no modal is open. Mode is one of the GoalEditMode* constants.
type GoalEdit struct {
	ID   string
	Mode string
}

// capturedGoalEdit holds the atom reference captured during a render so SetGoalEdit /
// CloseGoalEdit can update it from a click handler without calling state.UseAtom outside
// a render (which panics). Same convention as the budget/account editor atoms.
var (
	capturedGoalEdit state.Atom[GoalEdit]
	goalEditCaptured bool
)

// UseGoalEdit returns the shared atom selecting which goal editor modal is open. The
// goal card's Edit / Contribute buttons set it; the shell-mounted GoalEditHost reads it
// and renders the matching form inside a flip modal (rather than an inline row form,
// which sat under transformed bento/tile ancestors and rendered off-centre). Calling it
// in a render also captures the atom for SetGoalEdit/CloseGoalEdit.
func UseGoalEdit() state.Atom[GoalEdit] {
	a := state.UseAtom(goalEditAtomID, GoalEdit{})
	capturedGoalEdit = a
	goalEditCaptured = true
	return a
}

// SetGoalEdit opens the goal editor modal for the given goal + mode. Safe to call from a
// click handler (uses the captured atom, not UseAtom).
func SetGoalEdit(e GoalEdit) {
	if goalEditCaptured {
		capturedGoalEdit.Set(e)
	}
}

// CloseGoalEdit clears the goal-editor atom (dismisses the modal).
func CloseGoalEdit() { SetGoalEdit(GoalEdit{}) }
