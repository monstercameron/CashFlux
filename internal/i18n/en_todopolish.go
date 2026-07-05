// SPDX-License-Identifier: MIT

package i18n

// todoPolishKeys holds v1.0 To-do copy: the success toast shown after adding a
// task (the add form never confirmed, and the list didn't refresh). Merged via
// init so this file does not touch en.go.
var todoPolishKeys = Catalog{
	"todo.taskAdded": "Task added.",
}

func init() {
	for k, v := range todoPolishKeys {
		english[k] = v
	}
}
