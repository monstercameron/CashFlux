// SPDX-License-Identifier: MIT

package i18n

// todoPolishKeys holds English strings added by the 2026-07-19 To-do workspace
// polish pass: the quick-view lens (All / Today / Overdue), the standardized
// command bar's "More tools" menu, and the relocated, collapsible "Suggested for
// you" section that now sits BELOW the user's committed tasks. Merged via init so
// the shared en.go is never touched by this concurrent lane.
var todoPolishKeys = Catalog{
	// Quick-view segmented control — a coarse "what needs attention now" lens. (Named
	// lens* to stay clear of the add-form's todo.quick* due-date presets.)
	"todo.lensLabel":   "Quick view — show all tasks, just today's, or overdue",
	"todo.lensAll":     "All",
	"todo.lensToday":   "Today",
	"todo.lensOverdue": "Overdue",

	// The command bar's consolidated "More tools" overflow (uncommon actions:
	// the checklist templates).
	"todo.moreTools": "More tools",

	// The relocated "Suggested for you" section header. %d = how many suggestions
	// are waiting. It sits below the committed list and starts collapsed.
	"todo.suggestForYou": "Suggested for you (%d)",

	// Confirmation toast shown after the add-task form posts a new task. Referenced by
	// taskaddform.go; added here (a To-do-surface key) so it resolves instead of
	// rendering the raw key.
	"todo.taskAdded": "Task added",
}

func init() {
	for k, v := range todoPolishKeys {
		english[k] = v
	}
}
