// SPDX-License-Identifier: MIT

package tasktree

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasksort"
)

// roots builds N flat root tasks r1..rN (alphabetical titles so AZ order is predictable).
func roots(titles ...string) []domain.Task {
	out := make([]domain.Task, len(titles))
	for i, ti := range titles {
		out[i] = task("r"+ti, "", ti)
	}
	return out
}

func TestPagePaginatesRootsKeepingSubtrees(t *testing.T) {
	// Three roots (A, B, C alphabetically), B has two children. pageSize 2 by AZ.
	tasks := []domain.Task{
		task("rc", "", "C"),
		task("ra", "", "A"),
		task("rb", "", "B"),
		task("b1", "rb", "B-child-1"),
		task("b2", "rb", "B-child-2"),
	}
	// Page 1 (AZ, size 2): roots A, B → A, then B + its two children.
	nodes, total := Page(tasks, tasksort.ModeAZ, 1, 2)
	if total != 3 {
		t.Fatalf("totalRoots = %d, want 3", total)
	}
	if got, want := order(nodes), []string{"ra", "rb", "b1", "b2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("page1 = %v, want %v (subtree stays with its root)", got, want)
	}
	// Page 2: root C only.
	nodes, _ = Page(tasks, tasksort.ModeAZ, 2, 2)
	if got, want := order(nodes), []string{"rc"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("page2 = %v, want %v", got, want)
	}
}

func TestPageClampsOutOfRange(t *testing.T) {
	tasks := roots("A", "B", "C")
	// page 0 clamps to 1.
	n0, _ := Page(tasks, tasksort.ModeAZ, 0, 2)
	if got := order(n0); !reflect.DeepEqual(got, []string{"rA", "rB"}) {
		t.Fatalf("page 0 = %v, want first page", got)
	}
	// page 99 clamps to the last page (C).
	n9, total := Page(tasks, tasksort.ModeAZ, 99, 2)
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if got := order(n9); !reflect.DeepEqual(got, []string{"rC"}) {
		t.Fatalf("page 99 = %v, want last page [rC]", got)
	}
}

func TestPageNoPagingWhenSizeZero(t *testing.T) {
	tasks := roots("A", "B", "C", "D")
	nodes, total := Page(tasks, tasksort.ModeAZ, 1, 0)
	if total != 4 || len(nodes) != 4 {
		t.Fatalf("no-paging: total=%d nodes=%d, want 4/4", total, len(nodes))
	}
}

func TestPageOrdersByMode(t *testing.T) {
	tasks := []domain.Task{
		{ID: "lo", Title: "z-low", Status: domain.StatusOpen, Priority: domain.PriorityLow},
		{ID: "hi", Title: "a-high", Status: domain.StatusOpen, Priority: domain.PriorityHigh},
	}
	// Priority mode: high root first regardless of title.
	nodes, _ := Page(tasks, tasksort.ModePriority, 1, 10)
	if got := order(nodes); !reflect.DeepEqual(got, []string{"hi", "lo"}) {
		t.Fatalf("priority order = %v, want [hi lo]", got)
	}
	// AZ mode: a-high before z-low by title.
	nodes, _ = Page(tasks, tasksort.ModeAZ, 1, 10)
	if got := order(nodes); !reflect.DeepEqual(got, []string{"hi", "lo"}) {
		t.Fatalf("az order = %v, want [hi lo]", got)
	}
}

func TestPageCycleOrphanNotDropped(t *testing.T) {
	// a↔b cycle (each other's parent, both present) → neither is a natural root; they
	// must still render (folded in as roots), never dropped.
	tasks := []domain.Task{
		task("ok", "", "root"),
		task("a", "b", "cycle-a"),
		task("b", "a", "cycle-b"),
	}
	nodes, total := Page(tasks, tasksort.ModeSmart, 1, 0)
	if len(nodes) != 3 {
		t.Fatalf("cycle: emitted %d nodes, want 3 (nothing dropped): %v", len(nodes), order(nodes))
	}
	if total < 1 {
		t.Fatalf("cycle: totalRoots = %d, want >= 1", total)
	}
}

func TestPageDoesNotMutateInput(t *testing.T) {
	tasks := roots("C", "A", "B")
	before := order2(tasks)
	Page(tasks, tasksort.ModeAZ, 1, 2)
	if got := order2(tasks); !reflect.DeepEqual(got, before) {
		t.Fatalf("input mutated: %v → %v", before, got)
	}
}

func order2(ts []domain.Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}
