// SPDX-License-Identifier: MIT

package i18n

// recurTasksKeys holds the English strings for the recurring-task reminder
// controls on the To-do page: the "Remind me" lead select in the add/edit forms
// and the reminder chip shown on a task row. Defined in their own file and merged
// via init so this does not touch the shared en.go (mirrors en_billspaid.go). The
// repeat/cadence labels already live in en.go; only the reminder-lead strings are
// new here.
var recurTasksKeys = Catalog{
	// Label for the reminder-lead select, shown once a task is set to repeat.
	"todo.remind": "Remind me",
	// Reminder-lead options (how many days before the due date to start nudging).
	"todo.remindOnDue": "On the due date",
	"todo.remind1Day":  "1 day before",
	"todo.remind3Days": "3 days before",
	"todo.remind1Week": "1 week before",
	// Fallback reminder-chip / summary text for a non-preset positive lead
	// (e.g. "5d early"); the 1 / 3 / 7-day leads use the friendly phrasings above.
	"todo.reminderBadgeLead": "%s early",
}

func init() {
	for k, v := range recurTasksKeys {
		english[k] = v
	}
}
