// SPDX-License-Identifier: MIT

package tasksort

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func mkT(id string, order int, parent string) domain.Task {
	return domain.Task{ID: id, Title: id, Status: domain.StatusOpen, ParentID: parent, Order: order}
}

func orderOf(tasks []domain.Task, id string) (int, bool) {
	for _, t := range tasks {
		if t.ID == id {
			return t.Order, true
		}
	}
	return 0, false
}

func TestModeManualSortsByOrder(t *testing.T) {
	in := []domain.Task{mkT("c", 2, ""), mkT("a", 0, ""), mkT("b", 1, "")}
	got := OrderBy(in, ModeManual)
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if got[i].ID != w {
			t.Fatalf("manual order = %v, want %v", ids(got), want)
		}
	}
}

func TestReorderMovesToTargetSlot(t *testing.T) {
	tasks := []domain.Task{mkT("a", 0, ""), mkT("b", 1, ""), mkT("c", 2, "")}
	// Drag C onto A → C takes A's slot: [c, a, b].
	changed, ok := Reorder(tasks, "c", "a")
	if !ok {
		t.Fatal("Reorder(c,a) should succeed")
	}
	// Apply the changes to a copy and verify the resulting manual order.
	applied := applyChanges(tasks, changed)
	got := ids(OrderBy(applied, ModeManual))
	if want := []string{"c", "a", "b"}; !eq(got, want) {
		t.Fatalf("after reorder = %v, want %v", got, want)
	}
}

func TestReorderAdjacentOnlyChangesTwo(t *testing.T) {
	tasks := []domain.Task{mkT("a", 0, ""), mkT("b", 1, ""), mkT("c", 2, "")}
	// Drag B onto A → [b, a, c]. Only A and B move; C stays at 2.
	changed, ok := Reorder(tasks, "b", "a")
	if !ok {
		t.Fatal("should succeed")
	}
	if _, moved := orderOf(changed, "c"); moved {
		t.Fatal("C should not change")
	}
	applied := applyChanges(tasks, changed)
	if got := ids(OrderBy(applied, ModeManual)); !eq(got, []string{"b", "a", "c"}) {
		t.Fatalf("got %v, want [b a c]", got)
	}
}

func TestReorderRejectsNonSiblingsAndSelf(t *testing.T) {
	tasks := []domain.Task{mkT("a", 0, ""), mkT("b", 0, "p1")}
	if _, ok := Reorder(tasks, "a", "b"); ok {
		t.Fatal("cross-parent reorder should fail")
	}
	if _, ok := Reorder(tasks, "a", "a"); ok {
		t.Fatal("self reorder should fail")
	}
	if _, ok := Reorder(tasks, "a", "missing"); ok {
		t.Fatal("missing target should fail")
	}
}

func TestReorderWithinParentGroup(t *testing.T) {
	tasks := []domain.Task{
		mkT("root", 0, ""),
		mkT("s1", 0, "root"), mkT("s2", 1, "root"), mkT("s3", 2, "root"),
	}
	// Drag s3 onto s1 → sub-order [s3, s1, s2]; the root is untouched.
	changed, ok := Reorder(tasks, "s3", "s1")
	if !ok {
		t.Fatal("subtask reorder should succeed")
	}
	if _, moved := orderOf(changed, "root"); moved {
		t.Fatal("root must not move")
	}
	applied := applyChanges(tasks, changed)
	subs := []domain.Task{}
	for _, tk := range applied {
		if tk.ParentID == "root" {
			subs = append(subs, tk)
		}
	}
	if got := ids(OrderBy(subs, ModeManual)); !eq(got, []string{"s3", "s1", "s2"}) {
		t.Fatalf("sub order = %v, want [s3 s1 s2]", got)
	}
}

func applyChanges(tasks, changed []domain.Task) []domain.Task {
	out := make([]domain.Task, len(tasks))
	copy(out, tasks)
	for _, c := range changed {
		for i := range out {
			if out[i].ID == c.ID {
				out[i].Order = c.Order
			}
		}
	}
	return out
}
