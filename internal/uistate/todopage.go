// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

// taskEditAtomID keys the shared task-editor modal selector.
const taskEditAtomID = "todo:edit"

// TaskEdit selects which task the shell-root editor modal should show. A zero value
// (empty ID) means no modal is open.
type TaskEdit struct {
	ID string
}

// capturedTaskEdit holds the atom reference captured during a render so SetTaskEdit /
// CloseTaskEdit can update it from a click handler without calling state.UseAtom outside
// a render (which panics). Same convention as the goal/budget/account editor atoms.
var (
	capturedTaskEdit state.Atom[TaskEdit]
	taskEditCaptured bool
)

// UseTaskEdit returns the shared atom selecting which task editor modal is open. A task
// row's Edit button sets it; the shell-mounted TaskEditHost reads it and renders the
// edit form inside a flip modal (instead of an inline row form, which sat under
// transformed tile ancestors). Calling it in a render also captures the atom for
// SetTaskEdit/CloseTaskEdit.
func UseTaskEdit() state.Atom[TaskEdit] {
	a := state.UseAtom(taskEditAtomID, TaskEdit{})
	capturedTaskEdit = a
	taskEditCaptured = true
	return a
}

// SetTaskEdit opens the task editor modal for the given task. Safe to call from a click
// handler (uses the captured atom, not UseAtom).
func SetTaskEdit(e TaskEdit) {
	if taskEditCaptured {
		capturedTaskEdit.Set(e)
	}
}

// CloseTaskEdit clears the task-editor atom (dismisses the modal).
func CloseTaskEdit() { SetTaskEdit(TaskEdit{}) }
