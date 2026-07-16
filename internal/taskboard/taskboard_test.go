// SPDX-License-Identifier: MIT

package taskboard

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// keys extracts the ordered column keys for a set of columns.
func keys(cols []Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Key
	}
	return out
}

// titles extracts the per-column task titles in order.
func titles(c Column) []string {
	out := make([]string, len(c.Tasks))
	for i, t := range c.Tasks {
		out[i] = t.Title
	}
	return out
}

func TestColumnsByStatus(t *testing.T) {
	tasks := []domain.Task{
		{ID: "1", Title: "Pay rent", Status: domain.StatusOpen, Priority: domain.PriorityHigh},
		{ID: "2", Title: "File taxes", Status: domain.StatusDone, Priority: domain.PriorityLow},
		{ID: "3", Title: "Call bank", Status: domain.StatusOpen, Priority: domain.PriorityMedium},
		{ID: "4", Title: "Legacy", Status: domain.TaskStatus(""), Priority: domain.PriorityLow}, // empty → To do
	}
	cols := Columns(tasks, GroupByStatus)
	if got, want := keys(cols), []string{"open", "done"}; !equal(got, want) {
		t.Fatalf("status column keys = %v, want %v (Done must be last)", got, want)
	}
	if cols[0].Title != "todoboard.colTodo" || cols[1].Title != "todoboard.colDone" {
		t.Fatalf("unexpected column titles: %q, %q", cols[0].Title, cols[1].Title)
	}
	if got := len(cols[0].Tasks); got != 3 {
		t.Fatalf("To do column = %d tasks, want 3 (incl. empty-status fallback)", got)
	}
	if got := len(cols[1].Tasks); got != 1 {
		t.Fatalf("Done column = %d tasks, want 1", got)
	}
}

func TestColumnsByPriority(t *testing.T) {
	tasks := []domain.Task{
		{ID: "1", Title: "Low one", Status: domain.StatusOpen, Priority: domain.PriorityLow},
		{ID: "2", Title: "High one", Status: domain.StatusOpen, Priority: domain.PriorityHigh},
		{ID: "3", Title: "Med one", Status: domain.StatusOpen, Priority: domain.PriorityMedium},
		{ID: "4", Title: "Unset prio", Status: domain.StatusOpen, Priority: domain.TaskPriority("")}, // → Medium
	}
	cols := Columns(tasks, GroupByPriority)
	if got, want := keys(cols), []string{"high", "med", "low"}; !equal(got, want) {
		t.Fatalf("priority column keys = %v, want %v (high→low)", got, want)
	}
	if got := len(cols[0].Tasks); got != 1 { // high
		t.Fatalf("High column = %d, want 1", got)
	}
	if got := len(cols[1].Tasks); got != 2 { // med + unset fallback
		t.Fatalf("Medium column = %d, want 2 (incl. empty-priority fallback)", got)
	}
	if got := len(cols[2].Tasks); got != 1 { // low
		t.Fatalf("Low column = %d, want 1", got)
	}
}

func TestColumnsEmptyInput(t *testing.T) {
	for _, by := range []GroupBy{GroupByStatus, GroupByPriority} {
		cols := Columns(nil, by)
		if len(cols) == 0 {
			t.Fatalf("expected fixed columns even for empty input (by=%d)", by)
		}
		for _, c := range cols {
			if len(c.Tasks) != 0 {
				t.Fatalf("column %q should be empty for empty input, got %d", c.Key, len(c.Tasks))
			}
		}
	}
}

func TestWithinColumnSortDueThenTitle(t *testing.T) {
	tasks := []domain.Task{
		{ID: "a", Title: "Zebra", Status: domain.StatusOpen, Due: d(2026, 3, 10)},
		{ID: "b", Title: "Apple", Status: domain.StatusOpen, Due: d(2026, 1, 5)},
		{ID: "c", Title: "Beta", Status: domain.StatusOpen},                      // no due date → last
		{ID: "d", Title: "Alpha", Status: domain.StatusOpen},                     // no due date → last, Title tiebreak
		{ID: "e", Title: "Delta", Status: domain.StatusOpen, Due: d(2026, 1, 5)}, // same due as b → Title tiebreak
	}
	cols := Columns(tasks, GroupByStatus)
	got := titles(cols[0]) // the To do column
	// b & e share the earliest due (Title tiebreak: Apple < Delta), then a (later
	// due), then the undated ones by Title (Alpha < Beta).
	want := []string{"Apple", "Delta", "Zebra", "Alpha", "Beta"}
	if !equal(got, want) {
		t.Fatalf("within-column order = %v, want %v", got, want)
	}
}

func TestSortIsStable(t *testing.T) {
	// Two undated tasks with identical titles must keep input order (ID a before b).
	tasks := []domain.Task{
		{ID: "a", Title: "Same", Status: domain.StatusOpen},
		{ID: "b", Title: "Same", Status: domain.StatusOpen},
	}
	cols := Columns(tasks, GroupByStatus)
	if cols[0].Tasks[0].ID != "a" || cols[0].Tasks[1].ID != "b" {
		t.Fatalf("stable sort violated: got %s,%s", cols[0].Tasks[0].ID, cols[0].Tasks[1].ID)
	}
}

func TestNextKey(t *testing.T) {
	cases := []struct {
		by      GroupBy
		cur     string
		wantKey string
		wantOK  bool
	}{
		{GroupByStatus, "open", "done", true},
		{GroupByStatus, "done", "", false},
		{GroupByStatus, "bogus", "", false},
		{GroupByPriority, "high", "med", true},
		{GroupByPriority, "med", "low", true},
		{GroupByPriority, "low", "", false},
	}
	for _, c := range cases {
		gotKey, gotOK := NextKey(c.by, c.cur)
		if gotKey != c.wantKey || gotOK != c.wantOK {
			t.Errorf("NextKey(%d,%q) = %q,%v; want %q,%v", c.by, c.cur, gotKey, gotOK, c.wantKey, c.wantOK)
		}
	}
}

func equal(a, b []string) bool {
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
