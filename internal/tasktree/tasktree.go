// Package tasktree arranges flat to-do tasks into their parent/child hierarchy
// for display (C72). It is pure (no syscall/js) and table-tested: it flattens the
// tree into a depth-tagged render order and computes a task's descendants for
// cascade delete. Sibling order comes from internal/tasksort.
package tasktree

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasksort"
)

// Node is one task plus its nesting depth (0 = top level).
type Node struct {
	Task  domain.Task
	Depth int
}

// Flatten returns the given tasks in nested render order: each root followed by
// its descendants, depth-first, with siblings ordered by tasksort.Order. A task
// is a root when its ParentID is empty OR points to a task not in the set (e.g.
// the parent was filtered out by a hide-done view), so the input can be a
// pre-filtered subset and orphans still surface. Cycle-safe (each task is emitted
// at most once).
func Flatten(tasks []domain.Task) []Node {
	present := make(map[string]bool, len(tasks))
	children := make(map[string][]domain.Task, len(tasks))
	for _, t := range tasks {
		present[t.ID] = true
	}
	var roots []domain.Task
	for _, t := range tasks {
		if t.ParentID == "" || !present[t.ParentID] {
			roots = append(roots, t)
		} else {
			children[t.ParentID] = append(children[t.ParentID], t)
		}
	}

	out := make([]Node, 0, len(tasks))
	emitted := make(map[string]bool, len(tasks))
	var walk func(t domain.Task, depth int)
	walk = func(t domain.Task, depth int) {
		if emitted[t.ID] {
			return
		}
		emitted[t.ID] = true
		out = append(out, Node{Task: t, Depth: depth})
		for _, c := range tasksort.Order(children[t.ID]) {
			walk(c, depth+1)
		}
	}
	for _, r := range tasksort.Order(roots) {
		walk(r, 0)
	}
	// Fallback: any task not reached (e.g. caught in a parent cycle with no root)
	// still renders, as a top-level node — never silently drop a task.
	for _, t := range tasks {
		if !emitted[t.ID] {
			walk(t, 0)
		}
	}
	return out
}

// Descendants returns the ids of every task nested under id (children, grandchildren,
// …), for cascade delete. The id itself is not included. Cycle-safe.
func Descendants(tasks []domain.Task, id string) []string {
	children := make(map[string][]string)
	for _, t := range tasks {
		if t.ParentID != "" {
			children[t.ParentID] = append(children[t.ParentID], t.ID)
		}
	}
	var out []string
	seen := make(map[string]bool)
	var walk func(pid string)
	walk = func(pid string) {
		for _, c := range children[pid] {
			if seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
			walk(c)
		}
	}
	walk(id)
	return out
}
