// SPDX-License-Identifier: MIT

package taskchecklist

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestInstantiate(t *testing.T) {
	due := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	n := 0
	newID := func() string { n++; return fmt.Sprintf("id-%d", n) }

	items := []Item{
		{Title: "Reconcile accounts", DueOffsetDays: -3},
		{Title: "Review transactions"},
		{Title: "Snapshot reports", DueOffsetDays: 1},
	}
	got := Instantiate("Month-end close", items, due, newID)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4 (parent + 3 steps)", len(got))
	}
	parent := got[0]
	if parent.Title != "Month-end close" || parent.ParentID != "" || !parent.Due.Equal(due) {
		t.Errorf("parent = %+v", parent)
	}
	if parent.Status != domain.StatusOpen {
		t.Errorf("parent status = %s", parent.Status)
	}
	for i, child := range got[1:] {
		if child.ParentID != parent.ID {
			t.Errorf("step %d ParentID = %q, want %q", i, child.ParentID, parent.ID)
		}
		if child.Order != i+1 {
			t.Errorf("step %d Order = %d, want %d", i, child.Order, i+1)
		}
		if child.Title != items[i].Title {
			t.Errorf("step %d title = %q", i, child.Title)
		}
		want := due.AddDate(0, 0, items[i].DueOffsetDays)
		if !child.Due.Equal(want) {
			t.Errorf("step %d due = %v, want %v", i, child.Due, want)
		}
	}
	// IDs are unique.
	seen := map[string]bool{}
	for _, task := range got {
		if seen[task.ID] {
			t.Errorf("duplicate id %s", task.ID)
		}
		seen[task.ID] = true
	}
}

func TestInstantiateEmptyItems(t *testing.T) {
	got := Instantiate("Solo", nil, time.Now(), func() string { return "x" })
	if len(got) != 1 {
		t.Fatalf("len = %d, want just the parent", len(got))
	}
}
