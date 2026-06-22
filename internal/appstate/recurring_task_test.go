package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
)

func TestCompleteTask_NonRecurring(t *testing.T) {
	a := newApp(t, false)
	task := domain.Task{
		ID:       id.New(),
		Title:    "one-shot",
		Status:   domain.StatusOpen,
		Priority: domain.PriorityMedium,
		Source:   domain.SourceManual,
	}
	if err := a.PutTask(task); err != nil {
		t.Fatalf("PutTask: %v", err)
	}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := a.CompleteTask(task.ID, id.New(), now); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	tasks := a.Tasks()
	if len(tasks) != 1 {
		t.Errorf("want 1 task after completing non-recurring, got %d", len(tasks))
	}
	if tasks[0].Status != domain.StatusDone {
		t.Errorf("task Status = %v, want StatusDone", tasks[0].Status)
	}
}

func TestCompleteTask_Recurring_SpawnsNext(t *testing.T) {
	a := newApp(t, false)
	due := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	task := domain.Task{
		ID:         id.New(),
		Title:      "rebalance 401k",
		Status:     domain.StatusOpen,
		Priority:   domain.PriorityMedium,
		Source:     domain.SourceManual,
		Recurrence: domain.CadenceMonthly,
		Due:        due,
	}
	if err := a.PutTask(task); err != nil {
		t.Fatalf("PutTask: %v", err)
	}
	nextID := id.New()
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if err := a.CompleteTask(task.ID, nextID, now); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	tasks := a.Tasks()
	if len(tasks) != 2 {
		t.Fatalf("want 2 tasks (done + next), got %d", len(tasks))
	}

	byID := make(map[string]domain.Task, len(tasks))
	for _, tt := range tasks {
		byID[tt.ID] = tt
	}

	orig, ok := byID[task.ID]
	if !ok {
		t.Fatal("original task missing")
	}
	if orig.Status != domain.StatusDone {
		t.Errorf("original Status = %v, want StatusDone", orig.Status)
	}

	next, ok := byID[nextID]
	if !ok {
		t.Fatal("spawned task missing")
	}
	if next.Status != domain.StatusOpen {
		t.Errorf("next Status = %v, want StatusOpen", next.Status)
	}
	wantDue := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !next.Due.Equal(wantDue) {
		t.Errorf("next Due = %v, want %v", next.Due, wantDue)
	}
	if next.Recurrence != domain.CadenceMonthly {
		t.Errorf("next Recurrence = %v, want monthly", next.Recurrence)
	}
	if next.Title != task.Title {
		t.Errorf("next Title = %q, want %q", next.Title, task.Title)
	}
}

func TestCompleteTask_NotFound(t *testing.T) {
	a := newApp(t, false)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	err := a.CompleteTask("does-not-exist", id.New(), now)
	if err == nil {
		t.Error("expected error for missing task, got nil")
	}
}
