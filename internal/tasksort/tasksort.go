// Package tasksort orders and filters to-do tasks for display. It is the pure,
// table-tested home of the list rules that previously lived inline in the js-only
// to-do screen: open tasks first, then soonest due (dated before undated), then
// title; with an optional "hide done" filter.
package tasksort

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Order returns a copy of tasks sorted for the to-do list: open tasks before
// done ones, then by due date (dated tasks before undated, earliest first),
// then by title. The input slice is not modified.
func Order(tasks []domain.Task) []domain.Task {
	out := make([]domain.Task, len(tasks))
	copy(out, tasks)
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := out[i], out[j]
		if (ti.Status == domain.StatusOpen) != (tj.Status == domain.StatusOpen) {
			return ti.Status == domain.StatusOpen
		}
		if ti.Due.IsZero() != tj.Due.IsZero() {
			return !ti.Due.IsZero()
		}
		if !ti.Due.Equal(tj.Due) {
			return ti.Due.Before(tj.Due)
		}
		return ti.Title < tj.Title
	})
	return out
}

// Visible returns tasks with done ones removed when hideDone is set; otherwise
// it returns tasks unchanged. When filtering it allocates a new slice, so the
// input is never modified.
func Visible(tasks []domain.Task, hideDone bool) []domain.Task {
	if !hideDone {
		return tasks
	}
	out := make([]domain.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Status != domain.StatusDone {
			out = append(out, t)
		}
	}
	return out
}
