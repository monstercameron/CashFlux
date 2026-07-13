// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// goalEditAtomID keys the shared goal-editor modal selector.
const goalEditAtomID = "goals:edit"

// Goal-editor modes (which form the shell-root flip modal shows).
const (
	GoalEditModeEdit       = "edit"       // full edit form (name/target/date/owner/links/review cadence)
	GoalEditModeContribute = "contribute" // add an amount toward the goal (optionally posting to the ledger)
	GoalEditModeAllocate   = "allocate"   // virtual allocation: earmark account balances toward the goal (no txn)
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

// Goal sort keys the active-goals list can be ordered by (the toolbar's Sort control
// sets the atom; the list reads it). "actionable" is the default (nearest deadline →
// highest %); the others let the user surface what matters: closest to done, furthest
// away, most steps to work through, soonest deadline, or by name.
const (
	GoalSortActionable = "actionable" // nearest deadline, then highest % (default)
	GoalSortClosest    = "closest"    // highest % complete first (nearly there)
	GoalSortFarthest   = "farthest"   // lowest % complete first (just getting started)
	GoalSortComplexity = "complexity" // most linked steps (to-dos) first
	GoalSortDeadline   = "deadline"   // soonest target date first
	GoalSortName       = "name"       // alphabetical
)

// UseGoalSort returns the shared atom holding the active-goals sort key (default
// GoalSortActionable). Ephemeral (resets on reload) — a transient view choice, like
// the budgets sort.
func UseGoalSort() state.Atom[string] { return state.UseAtom("goals:sort", GoalSortActionable) }

// Goals-page top-level views (the tab strip): the goal cards, or the earmarks manager.
const (
	GoalsViewGoals    = "goals"    // the goal cards (default)
	GoalsViewEarmarks = "earmarks" // full-CRUD manager for virtual allocations across all goals
)

// UseGoalsView returns the shared atom selecting the goals-page tab (cards vs earmarks
// manager). Ephemeral (resets on reload) — a transient view choice.
func UseGoalsView() state.Atom[string] { return state.UseAtom("goals:view", GoalsViewGoals) }
