// SPDX-License-Identifier: MIT

package tasksort

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func ids(tasks []domain.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestOrder(t *testing.T) {
	// done-early sorts after open-late; undated open sorts after dated open;
	// same due date breaks ties by title.
	tasks := []domain.Task{
		{ID: "done", Status: domain.StatusDone, Due: d(2026, time.January, 1)},
		{ID: "open-undated", Status: domain.StatusOpen},
		{ID: "open-jun3-b", Status: domain.StatusOpen, Due: d(2026, time.June, 3), Title: "Beta"},
		{ID: "open-jun3-a", Status: domain.StatusOpen, Due: d(2026, time.June, 3), Title: "Alpha"},
		{ID: "open-jun1", Status: domain.StatusOpen, Due: d(2026, time.June, 1)},
	}
	got := ids(Order(tasks))
	want := []string{"open-jun1", "open-jun3-a", "open-jun3-b", "open-undated", "done"}
	if !eq(got, want) {
		t.Errorf("Order = %v, want %v", got, want)
	}
}

func TestOrderDoesNotMutate(t *testing.T) {
	tasks := []domain.Task{
		{ID: "b", Status: domain.StatusOpen, Title: "B"},
		{ID: "a", Status: domain.StatusOpen, Title: "A"},
	}
	_ = Order(tasks)
	if tasks[0].ID != "b" || tasks[1].ID != "a" {
		t.Errorf("Order mutated the input: %v", ids(tasks))
	}
}

func TestVisible(t *testing.T) {
	tasks := []domain.Task{
		{ID: "open", Status: domain.StatusOpen},
		{ID: "done", Status: domain.StatusDone},
	}
	if got := ids(Visible(tasks, false)); !eq(got, []string{"open", "done"}) {
		t.Errorf("Visible(hideDone=false) = %v, want all", got)
	}
	if got := ids(Visible(tasks, true)); !eq(got, []string{"open"}) {
		t.Errorf("Visible(hideDone=true) = %v, want only open", got)
	}
}
