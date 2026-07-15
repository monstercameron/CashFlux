// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestTaskRecurrenceRoundTrip proves a recurring task's cadence and reminder lead
// survive an export/import round-trip losslessly. Both fields are plain JSON on
// the Task blob, so this guards against a regression in the serialization path.
func TestTaskRecurrenceRoundTrip(t *testing.T) {
	s := newStore(t)
	task := domain.Task{
		ID: "task-recur", Title: "Pay rent", Status: domain.StatusOpen,
		Priority:         domain.PriorityHigh,
		Recurrence:       domain.CadenceMonthly,
		ReminderLeadDays: 3,
	}
	if err := s.PutTask(task); err != nil {
		t.Fatalf("PutTask: %v", err)
	}

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, _ := s2.ListTasks()
	if len(got) != 1 {
		t.Fatalf("want 1 task, got %d", len(got))
	}
	if got[0].Recurrence != domain.CadenceMonthly {
		t.Errorf("Recurrence = %q, want monthly", got[0].Recurrence)
	}
	if got[0].ReminderLeadDays != 3 {
		t.Errorf("ReminderLeadDays = %d, want 3", got[0].ReminderLeadDays)
	}
}
