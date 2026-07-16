// SPDX-License-Identifier: MIT

package i18n

// todoBoardKeys holds the copy for the to-do board (kanban) view: the column
// header titles for the status and priority groupings, the group-by option
// labels the coordinator's switcher shows, the per-card "advance to next
// column" affordance, the per-column count caption, and the empty-column note.
// Merged via init so this file does not touch en.go.
var todoBoardKeys = Catalog{
	// Status-grouping column headers. "In progress" is reserved for a future
	// intermediate status (the domain models only open/done today); the board
	// renders To do + Done, but the key is kept so the switcher copy is complete.
	"todoboard.colTodo":       "To do",
	"todoboard.colInProgress": "In progress",
	"todoboard.colDone":       "Done",
	// Priority-grouping column headers (high → low).
	"todoboard.colHigh":   "High priority",
	"todoboard.colMedium": "Medium priority",
	"todoboard.colLow":    "Low priority",
	// Group-by switcher option labels (the coordinator owns the control).
	"todoboard.groupByStatus":   "Group by status",
	"todoboard.groupByPriority": "Group by priority",
	// Per-card one-click advance affordance (status grouping only, where it marks a
	// task done): label + accessible title (%s = the next column's title, e.g. "Done").
	"todoboard.next":      "Done",
	"todoboard.nextTitle": "Mark as %s",
	// Per-column count caption (%d = number of tasks in the column).
	"todoboard.count": "%d",
	// Accessible label for a column's task count (%d = count).
	"todoboard.countLabel": "%d tasks",
	// Shown in a column with no tasks.
	"todoboard.emptyColumn": "Nothing here",
	// Accessible label for the whole board region.
	"todoboard.boardLabel": "Task board",
}

func init() {
	for k, v := range todoBoardKeys {
		english[k] = v
	}
}
