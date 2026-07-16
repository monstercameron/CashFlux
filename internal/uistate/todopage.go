// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

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

// capturedTaskAddParent lets a task row's "New sub-task" button set the parent for the next
// add-task modal from a click handler, without calling state.UseAtom outside a render.
var (
	capturedTaskAddParent state.Atom[string]
	taskAddParentCaptured bool
)

// UseTaskAddParent returns the atom holding the parent task id for the add-task modal ("" =
// a top-level task). AddHost reads it to seed TaskAddForm.ParentID (so the full compose form
// — not a bare prompt — creates a sub-task); a row's "New sub-task" button sets it. Calling
// it in a render also captures the atom for SetTaskAddParent.
func UseTaskAddParent() state.Atom[string] {
	a := state.UseAtom("todo:addParent", "")
	capturedTaskAddParent = a
	taskAddParentCaptured = true
	return a
}

// SetTaskAddParent sets (or clears with "") the parent for the next add-task modal. Safe from
// a click handler (uses the captured atom, not UseAtom).
func SetTaskAddParent(parentID string) {
	if taskAddParentCaptured {
		capturedTaskAddParent.Set(parentID)
	}
}

// UseTodoHideDone is the shared "hide completed tasks" toggle. The widgetized to-do
// surface splits the controls (toolbar tile) from the list (list tile), so this state
// lives in an atom both tiles read — mirroring UseGoalsShowFormulas on /goals.
func UseTodoHideDone() state.Atom[bool] { return state.UseAtom("todo:hideDone", false) }

// UseTodoFilterPrio is the shared lightweight priority filter for the to-do surface
// ("" = all, or a domain.TaskPriority string). Read by both the toolbar and list tiles.
func UseTodoFilterPrio() state.Atom[string] { return state.UseAtom("todo:filterPrio", "") }

// UseTodoSortMode is the shared task ordering for the to-do surface (a tasksort.Mode
// string: "smart" / "priority" / "az" / "due"). Read by both the toolbar and list tiles.
func UseTodoSortMode() state.Atom[string] { return state.UseAtom("todo:sortMode", "smart") }

// UseTodoPageSize is the shared rows-per-page for the to-do list (default 20; a value <= 0
// means "show all"). The pager's rows-per-page buttons set it; the list slices by it.
func UseTodoPageSize() state.Atom[int] { return state.UseAtom("todo:pageSize", 20) }

// UseTodoSearch is the shared free-text search for the to-do surface ("" = no search).
// The toolbar's search box sets it; the list tile filters task titles (and notes) by it.
func UseTodoSearch() state.Atom[string] { return state.UseAtom("todo:search", "") }

// Linked-feature filter values for the to-do surface. "" = all; "none" = only unlinked
// tasks; otherwise a domain.RelatedType string (goal / budget / account / transaction).
const (
	TodoLinkAll         = ""     // every task, linked or not
	TodoLinkNone        = "none" // only tasks not linked to any feature
	TodoLinkGoal        = "goal"
	TodoLinkBudget      = "budget"
	TodoLinkAccount     = "account"
	TodoLinkTransaction = "transaction"
)

// capturedTodoFilterLink lets another surface (e.g. a transaction's "N follow-ups" chip)
// deep-link into the to-do list pre-filtered to a link type, without calling UseAtom
// outside a render. Captured whenever UseTodoFilterLink runs — the to-do surface and
// the always-mounted AddHost both call it, so the ref is live app-wide.
var (
	capturedTodoFilterLink state.Atom[string]
	todoFilterLinkCaptured bool
)

// UseTodoFilterLink is the shared "linked to" filter for the to-do surface — show only
// tasks tied to a given feature (goals / budgets / accounts / transactions), only
// unlinked ones, or all. Read by both the toolbar and list tiles.
func UseTodoFilterLink() state.Atom[string] {
	a := state.UseAtom("todo:filterLink", TodoLinkAll)
	capturedTodoFilterLink = a
	todoFilterLinkCaptured = true
	return a
}

// SetTodoFilterLink sets the to-do "linked to" filter from a click handler on another
// page (uses the captured atom, not UseAtom). No-op until the atom has been captured.
func SetTodoFilterLink(v string) {
	if todoFilterLinkCaptured {
		capturedTodoFilterLink.Set(v)
	}
}

// capturedTodoFilterLinkID narrows the link filter to a SPECIFIC linked entity (e.g. one
// budget's follow-ups), on top of the link-type filter. "" = no id narrowing. Deep-linked
// from a budget card's to-do panel; cleared when the user changes the link-type dropdown.
var (
	capturedTodoFilterLinkID state.Atom[string]
	todoFilterLinkIDCaptured bool
)

// UseTodoFilterLinkID is the shared "linked to this specific entity id" narrowing for the
// to-do surface (applied together with the link-type filter). Read by the list tile.
func UseTodoFilterLinkID() state.Atom[string] {
	a := state.UseAtom("todo:filterLinkID", "")
	capturedTodoFilterLinkID = a
	todoFilterLinkIDCaptured = true
	return a
}

// SetTodoFilterLinkID sets (or clears with "") the specific-entity narrowing from another
// page. No-op until the atom has been captured.
func SetTodoFilterLinkID(id string) {
	if todoFilterLinkIDCaptured {
		capturedTodoFilterLinkID.Set(id)
	}
}

// UseTodoPage is the shared 1-based current page for the to-do list (pagination is by
// top-level task, so sub-trees stay together). Reset to 1 when the sort/filter changes.
func UseTodoPage() state.Atom[int] {
	a := state.UseAtom("todo:page", 1)
	capturedTodoPage = a
	todoPageCaptured = true
	return a
}

var (
	capturedTodoPage state.Atom[int]
	todoPageCaptured bool
)

// ResetTodoPage returns the to-do list to page 1 from outside a render (e.g. the
// shell-root add form), so a just-added task lands on the visible top page
// rather than leaving the user on a later page. No-op until the list has
// rendered once (always true after first paint).
func ResetTodoPage() {
	if todoPageCaptured {
		capturedTodoPage.Set(1)
	}
}

// capturedTodoCollapsed lets ToggleTodoCollapsed flip a parent's collapse state from a
// row click handler without calling state.UseAtom outside a render (which panics).
var (
	capturedTodoCollapsed state.Atom[map[string]bool]
	todoCollapsedCaptured bool
)

// UseTodoCollapsed is the shared set of task IDs whose sub-tasks are collapsed (hidden).
// The list tile reads it to prune collapsed sub-trees; a parent row's disclosure toggle
// flips it via ToggleTodoCollapsed. Calling it in a render also captures the atom.
func UseTodoCollapsed() state.Atom[map[string]bool] {
	a := state.UseAtom("todo:collapsed", map[string]bool{})
	capturedTodoCollapsed = a
	todoCollapsedCaptured = true
	return a
}

// ToggleTodoCollapsed collapses/expands a parent task's sub-tasks. Safe from a click
// handler (uses the captured atom); copies the map so the change is a new value the
// atom's subscribers see.
func ToggleTodoCollapsed(id string) {
	if !todoCollapsedCaptured {
		return
	}
	cur := capturedTodoCollapsed.Get()
	nm := make(map[string]bool, len(cur)+1)
	for k, v := range cur {
		nm[k] = v
	}
	if nm[id] {
		delete(nm, id)
	} else {
		nm[id] = true
	}
	capturedTodoCollapsed.Set(nm)
}
