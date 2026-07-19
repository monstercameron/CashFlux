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

func TestFilterQuickView(t *testing.T) {
	today := d(2026, time.July, 19)
	todayISO := "2026-07-19"
	tasks := []domain.Task{
		{ID: "overdue", Status: domain.StatusOpen, Due: d(2026, time.July, 10)},
		{ID: "today", Status: domain.StatusOpen, Due: today},
		{ID: "future", Status: domain.StatusOpen, Due: d(2026, time.July, 25)},
		{ID: "undated", Status: domain.StatusOpen},
		{ID: "doneToday", Status: domain.StatusDone, Due: today},
		{ID: "doneOverdue", Status: domain.StatusDone, Due: d(2026, time.July, 1)},
	}
	// All is a passthrough (identity, including done/undated).
	if got := ids(FilterQuickView(tasks, QuickAll, todayISO)); !eq(got, ids(tasks)) {
		t.Errorf("QuickAll = %v, want unchanged", got)
	}
	// Today keeps only the open task due exactly today (not the done one).
	if got := ids(FilterQuickView(tasks, QuickToday, todayISO)); !eq(got, []string{"today"}) {
		t.Errorf("QuickToday = %v, want [today]", got)
	}
	// Overdue keeps only the open, past-due task.
	if got := ids(FilterQuickView(tasks, QuickOverdue, todayISO)); !eq(got, []string{"overdue"}) {
		t.Errorf("QuickOverdue = %v, want [overdue]", got)
	}
	// Empty todayISO is a passthrough guard.
	if got := ids(FilterQuickView(tasks, QuickToday, "")); !eq(got, ids(tasks)) {
		t.Errorf("QuickToday with empty todayISO = %v, want unchanged", got)
	}
	// The input slice is never mutated.
	if tasks[0].ID != "overdue" {
		t.Errorf("input slice was reordered/mutated")
	}
}

func TestCountQuickViews(t *testing.T) {
	todayISO := "2026-07-19"
	tasks := []domain.Task{
		{ID: "o1", Status: domain.StatusOpen, Due: d(2026, time.July, 10)},
		{ID: "o2", Status: domain.StatusOpen, Due: d(2026, time.July, 18)},
		{ID: "t1", Status: domain.StatusOpen, Due: d(2026, time.July, 19)},
		{ID: "f1", Status: domain.StatusOpen, Due: d(2026, time.July, 20)},
		{ID: "u1", Status: domain.StatusOpen},
		{ID: "d1", Status: domain.StatusDone, Due: d(2026, time.July, 1)},
	}
	c := CountQuickViews(tasks, todayISO)
	if c.Today != 1 || c.Overdue != 2 {
		t.Errorf("CountQuickViews = %+v, want {Today:1 Overdue:2}", c)
	}
	if empty := CountQuickViews(tasks, ""); empty.Today != 0 || empty.Overdue != 0 {
		t.Errorf("CountQuickViews empty todayISO = %+v, want zero", empty)
	}
}

func TestParseQuickView(t *testing.T) {
	cases := map[string]QuickView{
		"today": QuickToday, "overdue": QuickOverdue,
		"all": QuickAll, "": QuickAll, "bogus": QuickAll,
	}
	for in, want := range cases {
		if got := ParseQuickView(in); got != want {
			t.Errorf("ParseQuickView(%q) = %q, want %q", in, got, want)
		}
	}
}
