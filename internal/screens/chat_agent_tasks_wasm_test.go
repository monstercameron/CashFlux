// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func assistantTasksApp(t *testing.T, tasks ...domain.Task) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	for _, task := range tasks {
		if err := app.PutTask(task); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func assistantOpenTask(id, title string) domain.Task {
	return domain.Task{ID: id, Title: title, Status: domain.StatusOpen, Priority: domain.PriorityMedium}
}

func assistantCompleteTool(app *appstate.App) chatTool {
	return agToolsTasks(app, "USD", currency.Rates{Base: "USD"})[0]
}

func TestAssistantCompleteTaskRegisteredOnceAndMutating(t *testing.T) {
	app := assistantTasksApp(t)
	count := 0
	for _, tool := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull) {
		if tool.spec.Function.Name == "complete_task" {
			count++
			if !tool.mutates || tool.preview == nil || !strings.Contains(string(tool.spec.Function.Parameters), `"task_id"`) {
				t.Fatal("completion lost approval gating or exact-ID parameter")
			}
		}
	}
	if count != 1 {
		t.Fatalf("completion tool registrations = %d, want 1", count)
	}
}

func TestAssistantCompleteTaskCanonicalRecurrenceAndRetry(t *testing.T) {
	task := assistantOpenTask("monthly", "Review bills")
	task.Recurrence = domain.CadenceMonthly
	task.Due = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	task.Notes, task.ReminderLeadDays = "Keep the series", 3
	app := assistantTasksApp(t, task)
	tool := assistantCompleteTool(app)
	raw := json.RawMessage(`{"task_id":"monthly"}`)
	preview := tool.preview(raw)
	for _, want := range []string{"monthly", "Review bills", "next monthly", "2026-11-01"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview %q lacks %q", preview, want)
		}
	}
	if out := tool.run(raw); !strings.Contains(out, "Marked done") {
		t.Fatal(out)
	}
	tasks := app.Tasks()
	if len(tasks) != 2 {
		t.Fatalf("canonical completion produced %d tasks, want done + successor", len(tasks))
	}
	for _, got := range tasks {
		if got.ID == task.ID {
			if got.Status != domain.StatusDone {
				t.Fatal("original occurrence stayed open")
			}
		} else if got.Status != domain.StatusOpen || got.Recurrence != task.Recurrence || got.Title != task.Title ||
			got.Notes != task.Notes || got.ReminderLeadDays != task.ReminderLeadDays || !got.Due.Equal(task.Due.AddDate(0, 1, 0)) {
			t.Fatalf("incorrect canonical successor: %+v", got)
		}
	}
	revision := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already done") || !reflect.DeepEqual(app.Tasks(), tasks) || app.Rev() != revision {
		t.Fatalf("retry changed the recurring series: %q", out)
	}
	// A later call using only the recurring title must ask for an occurrence ID.
	later := assistantCompleteTool(app)
	if out := later.run(json.RawMessage(`{"title":"Review bills"}`)); !strings.Contains(out, "several to-dos") || app.Rev() != revision {
		t.Fatalf("title retry silently completed the next occurrence: %q", out)
	}
}

func TestAssistantCompleteTaskIdentityAndAmbiguousNames(t *testing.T) {
	for _, tc := range []struct {
		name, raw, target, want string
	}{
		{"exact ID overrides title", `{"task_id":"other","title":"Rent"}`, "other", "Marked done"},
		{"exact title before substring", `{"title":"rEnT"}`, "rent", "Marked done"},
		{"unique substring", `{"title":"insurance"}`, "other", "Marked done"},
		{"ambiguous names", `{"title":"review"}`, "", "several to-dos"},
		{"missing ID never falls back", `{"task_id":"missing","title":"Rent"}`, "", "no to-do with ID"},
		{"empty", `{}`, "", "provide a task_id or title"},
		{"malformed", `{"title":42}`, "", "couldn't read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := assistantTasksApp(t, assistantOpenTask("rent", "Rent"), assistantOpenTask("rent-review", "Review rent"),
				assistantOpenTask("other", "Review insurance"))
			tool := assistantCompleteTool(app)
			raw := json.RawMessage(tc.raw)
			before, rev := app.Tasks(), app.Rev()
			tool.preview(raw)
			out := tool.run(raw)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("result = %q, want %q", out, tc.want)
			}
			if tc.target == "" {
				if !reflect.DeepEqual(app.Tasks(), before) || app.Rev() != rev {
					t.Fatal("invalid/ambiguous selection changed tasks")
				}
			} else {
				for _, task := range app.Tasks() {
					if (task.Status == domain.StatusDone) != (task.ID == tc.target) {
						t.Fatalf("completed wrong occurrence: %+v", task)
					}
				}
			}
		})
	}
}

func TestAssistantCompleteTaskAlreadyDoneIsNoop(t *testing.T) {
	task := assistantOpenTask("done", "Completed review")
	task.Status, task.Recurrence = domain.StatusDone, domain.CadenceWeekly
	app := assistantTasksApp(t, task)
	tool := assistantCompleteTool(app)
	raw := json.RawMessage(`{"title":"Completed review"}`)
	before, rev := app.Tasks(), app.Rev()
	if out := tool.preview(raw); !strings.Contains(out, "already done") {
		t.Fatal(out)
	}
	if out := tool.run(raw); !strings.Contains(out, "already done") || !reflect.DeepEqual(app.Tasks(), before) || app.Rev() != rev {
		t.Fatalf("done occurrence was written or respawned: %q", out)
	}
}

func TestAssistantCompleteTaskApprovalCannotRetargetChangedTask(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed", true: "deleted"}[remove], func(t *testing.T) {
			task := assistantOpenTask("approved", "Review bills")
			app := assistantTasksApp(t, task)
			tool := assistantCompleteTool(app)
			raw := json.RawMessage(`{"title":"Review bills"}`)
			tool.preview(raw)
			if remove {
				if err := app.DeleteTask(task.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				task.Title = "Changed after approval"
				if err := app.PutTask(task); err != nil {
					t.Fatal(err)
				}
			}
			if err := app.PutTask(assistantOpenTask("replacement", "Review bills")); err != nil {
				t.Fatal(err)
			}
			before, rev := app.Tasks(), app.Rev()
			out := tool.run(raw)
			want := "changed after the preview"
			if remove {
				want = "no longer exists"
			}
			if !strings.Contains(out, want) || !reflect.DeepEqual(app.Tasks(), before) || app.Rev() != rev {
				t.Fatalf("approval silently retargeted/reinterpreted: %q", out)
			}
		})
	}
}

func TestAssistantCompleteTaskWriteFailureCanRetryOnce(t *testing.T) {
	task := assistantOpenTask("retry", "Review bills")
	task.Recurrence = domain.CadenceWeekly
	app := assistantTasksApp(t, task)
	tool := assistantCompleteTool(app)
	raw := json.RawMessage(`{"task_id":"retry"}`)
	tool.preview(raw)
	before, rev := app.Tasks(), app.Rev()
	app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	if out := tool.run(raw); !strings.Contains(out, "Couldn't complete") || !reflect.DeepEqual(app.Tasks(), before) || app.Rev() != rev {
		t.Fatalf("failed completion mutated data or hid the error: %q", out)
	}
	app.SetActiveRoleFunc(nil)
	if out := tool.run(raw); !strings.Contains(out, "Marked done") || len(app.Tasks()) != 2 {
		t.Fatalf("corrected retry failed: %q", out)
	}
	rev = app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already done") || len(app.Tasks()) != 2 || app.Rev() != rev {
		t.Fatalf("successful retry repeated the write: %q", out)
	}
}
