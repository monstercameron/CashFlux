// SPDX-License-Identifier: MIT

package i18n

// todoViewKeys holds the copy for the To-do surface view switcher (list / board /
// calendar) and the calendar schedule view. Merged via init so this file does not
// touch en.go.
var todoViewKeys = Catalog{
	// Segmented view switcher on the To-do toolbar.
	"todo.viewLabel":    "View",
	"todo.viewList":     "List",
	"todo.viewBoard":    "Board",
	"todo.viewCalendar": "Calendar",
	// Board group-by control (only shown in board view).
	"todo.boardGroupLabel":    "Group",
	"todo.boardGroupStatus":   "Status",
	"todo.boardGroupPriority": "Priority",
	// Calendar view.
	"todo.calendarLabel": "Task calendar",
	// Aria/title for a calendar day's "add a task on this day" affordance. %s = the date.
	"todo.calendarAddOnDay": "Add a task due %s",
}

func init() {
	for k, v := range todoViewKeys {
		english[k] = v
	}
}
