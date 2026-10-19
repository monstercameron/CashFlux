// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestCompleteTask_RecurringFailureLeavesBothTasksUnchanged(t *testing.T) {
	a := newApp(t, false)
	task := domain.Task{
		ID:         "recurring-task",
		Title:      "Review bills",
		Status:     domain.StatusOpen,
		Priority:   domain.PriorityMedium,
		Recurrence: domain.CadenceMonthly,
		Due:        time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := a.PutTask(task); err != nil {
		t.Fatalf("PutTask: %v", err)
	}

	// Reusing the source ID would overwrite it with the successor. The store
	// rejects the duplicate inside its transaction after staging the done row.
	if err := a.CompleteTask(task.ID, task.ID, time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("CompleteTask succeeded with a colliding successor ID, want error")
	}

	got := a.Tasks()
	if len(got) != 1 {
		t.Fatalf("tasks after failed completion = %d, want only original task", len(got))
	}
	if got[0].ID != task.ID || got[0].Status != domain.StatusOpen {
		t.Fatalf("original task after failure = %+v, want unchanged open task", got[0])
	}
}
