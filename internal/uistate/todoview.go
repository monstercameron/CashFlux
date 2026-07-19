// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// To-do surface view mode: the list (default), the kanban board, or the calendar
// schedule. Persisted only in-session (a plain atom) so the surface reopens on the
// familiar list; the switch is one click on the toolbar.
const (
	TodoViewList     = "list"
	TodoViewBoard    = "board"
	TodoViewCalendar = "calendar"
)

// UseTodoView returns the shared atom selecting the To-do surface view (list / board /
// calendar). Read by the toolbar (to render the segmented switch) and the list tile
// (to decide which body to render).
func UseTodoView() state.Atom[string] { return state.UseAtom("todo:view", TodoViewList) }

// UseTodoBoardGroup returns the board's group-by mode ("status" or "priority").
func UseTodoBoardGroup() state.Atom[string] { return state.UseAtom("todo:boardGroup", "status") }

// UseTodoQuickView returns the shared quick-view lens for the To-do workspace: "all"
// (default), "today" (open tasks due today), or "overdue" (open tasks past due) — a
// tasksort.QuickView string. The command bar renders it as a segmented control; the
// list tile narrows the visible set through tasksort.FilterQuickView. In-session only.
func UseTodoQuickView() state.Atom[string] { return state.UseAtom("todo:quickView", "all") }

// UseTodoSuggestOpen returns the expand/collapse state for the To-do workspace's
// "Suggested for you" section — the deterministic condition nudges, now shown in a
// clearly-labeled section BELOW the user's committed tasks (not above them). Default
// false = collapsed, so suggestions never crowd the user's own list; the labeled
// header expands them on demand. In-session only.
func UseTodoSuggestOpen() state.Atom[bool] { return state.UseAtom("todo:suggestOpen", false) }

// UseTodoCalOffset returns the calendar view's month offset from the current month
// (0 = this month, -1 = last month, +1 = next). The prev/next chevrons step it; it
// resets naturally each session.
func UseTodoCalOffset() state.Atom[int] { return state.UseAtom("todo:calOffset", 0) }

// capturedTaskAddDue lets the calendar view seed a due date for the next add-task modal
// from a click handler (click a day → new task due that day), without calling UseAtom
// outside a render. Mirrors the parent-preset seam in todopage.go.
var (
	capturedTaskAddDue state.Atom[string]
	taskAddDueCaptured bool
)

// UseTaskAddDue returns the atom holding a preset due date (ISO yyyy-mm-dd, "" = none)
// for the add-task modal. AddHost reads it to seed TaskAddForm.PresetDue; the calendar
// view sets it on a day click. Calling it in a render also captures the atom for
// SetTaskAddDue.
func UseTaskAddDue() state.Atom[string] {
	a := state.UseAtom("todo:addDue", "")
	capturedTaskAddDue = a
	taskAddDueCaptured = true
	return a
}

// SetTaskAddDue sets (or clears with "") the preset due date for the next add-task modal.
// Safe from a click handler (uses the captured atom, not UseAtom).
func SetTaskAddDue(iso string) {
	if taskAddDueCaptured {
		capturedTaskAddDue.Set(iso)
	}
}
