// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/taskrecur"
)

type assistantTaskCompletion struct {
	task domain.Task
	now  time.Time
	err  error
}

// resolveAssistantTask selects one occurrence, including completed occurrences so
// a retry by title cannot silently advance the next occurrence of a series.
func resolveAssistantTask(tasks []domain.Task, raw json.RawMessage) (domain.Task, error) {
	var args struct {
		TaskID string `json:"task_id"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return domain.Task{}, fmt.Errorf("couldn't read the task details: %w", err)
	}
	if taskID := strings.TrimSpace(args.TaskID); taskID != "" {
		for _, task := range tasks {
			if task.ID == taskID {
				return task, nil
			}
		}
		return domain.Task{}, fmt.Errorf("no to-do with ID %q", taskID)
	}
	query := strings.TrimSpace(args.Title)
	if query == "" {
		return domain.Task{}, fmt.Errorf("provide a task_id or title")
	}
	var exact, partial []domain.Task
	for _, task := range tasks {
		if strings.EqualFold(task.Title, query) {
			exact = append(exact, task)
		} else if strings.Contains(strings.ToLower(task.Title), strings.ToLower(query)) {
			partial = append(partial, task)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = partial
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return domain.Task{}, fmt.Errorf("no to-do matching %q", query)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	var choices []string
	for _, task := range matches {
		choices = append(choices, fmt.Sprintf("%q (ID %s, %s)", task.Title, task.ID, task.Status))
	}
	return domain.Task{}, fmt.Errorf("several to-dos match %q; provide the task_id: %s", query, strings.Join(choices, "; "))
}

// agToolsTasks supplies canonical completion with an approval-bound occurrence.
// Each preview binds the selected record and timestamp; execution checks that
// same ID and refuses edits made while the approval card was open.
func agToolsTasks(app *appstate.App, _ string, _ currency.Rates) []chatTool {
	prepared := map[string]assistantTaskCompletion{}
	prepare := func(raw json.RawMessage) assistantTaskCompletion {
		task, err := resolveAssistantTask(app.Tasks(), raw)
		value := assistantTaskCompletion{task: task, now: time.Now(), err: err}
		prepared[string(raw)] = value
		return value
	}
	return []chatTool{{
		spec: ai.FunctionTool("complete_task",
			"Mark one to-do occurrence done. Prefer its exact task_id; otherwise provide an exact or uniquely matching title. Ambiguous titles require a task_id. Recurring tasks create their next open occurrence; an already completed occurrence is unchanged.",
			json.RawMessage(`{"type":"object","properties":{"task_id":{"type":"string"},"title":{"type":"string"}},"anyOf":[{"required":["task_id"]},{"required":["title"]}]}`)),
		mutates: true,
		preview: func(raw json.RawMessage) string {
			value := prepare(raw)
			if value.err != nil {
				return "Can't complete that to-do: " + value.err.Error()
			}
			task := value.task
			if task.Status == domain.StatusDone {
				return fmt.Sprintf("To-do %q (ID %s) is already done; no change.", task.Title, task.ID)
			}
			out := fmt.Sprintf("Mark done: %q (ID %s).", task.Title, task.ID)
			if next, spawn := taskrecur.NextOccurrence(task, "", value.now); spawn {
				out += fmt.Sprintf(" Create its next %s occurrence, due %s.", task.Recurrence, next.Due.Format("2006-01-02"))
			}
			return out
		},
		run: func(raw json.RawMessage) string {
			value, ok := prepared[string(raw)]
			if !ok {
				value = prepare(raw)
			}
			if value.err != nil {
				return "Can't complete that to-do: " + value.err.Error()
			}
			var current domain.Task
			found := false
			for _, task := range app.Tasks() {
				if task.ID == value.task.ID {
					current, found = task, true
					break
				}
			}
			if !found {
				return fmt.Sprintf("To-do ID %s no longer exists; nothing changed.", value.task.ID)
			}
			if current.Status == domain.StatusDone {
				return fmt.Sprintf("To-do %q (ID %s) is already done; nothing changed.", current.Title, current.ID)
			}
			if !reflect.DeepEqual(current, value.task) {
				return fmt.Sprintf("To-do ID %s changed after the preview; review it again before completing it. Nothing changed.", current.ID)
			}
			if err := app.CompleteTask(current.ID, id.New(), value.now); err != nil {
				return "Couldn't complete the to-do: " + err.Error()
			}
			return fmt.Sprintf("Marked done: %s (ID %s).%s", current.Title, current.ID, openLink("/todo", current.ID))
		},
	}}
}
