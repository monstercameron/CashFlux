// SPDX-License-Identifier: MIT

// Package tasksort orders and filters to-do tasks for display. It is the pure,
// table-tested home of the list rules that previously lived inline in the js-only
// to-do screen: open tasks first, then soonest due (dated before undated), then
// title; with an optional "hide done" filter.
package tasksort

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Mode is a user-selectable ordering for a task list (e.g. the dashboard To-do
// widget). ModeSmart is the default list order from Order.
type Mode string

const (
	ModeSmart    Mode = "smart"    // open-first → soonest due → title (the Order default)
	ModePriority Mode = "priority" // high → low, then the smart tiebreak
	ModeAZ       Mode = "az"       // alphabetical by title
	ModeDue      Mode = "due"      // soonest due first (dated before undated)
	ModeManual   Mode = "manual"   // the user's drag-and-drop order (Task.Order), no reflow
)

// ParseMode returns a valid Mode, defaulting to ModeSmart for unknown input.
func ParseMode(s string) Mode {
	switch Mode(s) {
	case ModePriority, ModeAZ, ModeDue, ModeManual:
		return Mode(s)
	default:
		return ModeSmart
	}
}

// priorityRank ranks a task's priority for sorting (higher = more urgent).
func priorityRank(p domain.TaskPriority) int {
	switch p {
	case domain.PriorityHigh:
		return 3
	case domain.PriorityMedium:
		return 2
	case domain.PriorityLow:
		return 1
	default:
		return 0
	}
}

// OrderBy returns a copy of tasks sorted under the given mode. Every mode keeps
// open tasks ahead of done ones (a done task never outranks an open one), then
// applies the mode's primary key, falling back to the smart order for ties. The
// input slice is not modified.
func OrderBy(tasks []domain.Task, mode Mode) []domain.Task {
	if mode == ModeSmart {
		return Order(tasks)
	}
	out := make([]domain.Task, len(tasks))
	copy(out, tasks)
	// Manual mode is the user's own drag order — pure Task.Order, no open-first reflow, so a
	// task stays exactly where it was dropped (stable for equal Order values).
	if mode == ModeManual {
		sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := out[i], out[j]
		if oi, oj := ti.Status == domain.StatusOpen, tj.Status == domain.StatusOpen; oi != oj {
			return oi // open first
		}
		switch mode {
		case ModePriority:
			if ri, rj := priorityRank(ti.Priority), priorityRank(tj.Priority); ri != rj {
				return ri > rj
			}
		case ModeAZ:
			if li, lj := strings.ToLower(ti.Title), strings.ToLower(tj.Title); li != lj {
				return li < lj
			}
		case ModeDue:
			if ti.Due.IsZero() != tj.Due.IsZero() {
				return !ti.Due.IsZero()
			}
			if !ti.Due.Equal(tj.Due) {
				return ti.Due.Before(tj.Due)
			}
		}
		// Smart tiebreak.
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

// Reorder moves src to target's slot among their shared sibling group (tasks with the same
// ParentID), reassigning sequential Order values (0,1,2,…) to that whole group so the manual
// order is dense and stable. It returns the tasks whose Order changed (with Order updated) —
// the caller persists them — and ok=false when src==target, either id is missing, or the two
// are not siblings. Pure: the input slice is not modified.
func Reorder(tasks []domain.Task, srcID, targetID string) ([]domain.Task, bool) {
	if srcID == "" || targetID == "" || srcID == targetID {
		return nil, false
	}
	var src, target *domain.Task
	for i := range tasks {
		switch tasks[i].ID {
		case srcID:
			src = &tasks[i]
		case targetID:
			target = &tasks[i]
		}
	}
	if src == nil || target == nil || src.ParentID != target.ParentID {
		return nil, false
	}
	// The sibling group in current manual order.
	group := make([]domain.Task, 0)
	for _, t := range tasks {
		if t.ParentID == src.ParentID {
			group = append(group, t)
		}
	}
	sort.SliceStable(group, func(i, j int) bool { return group[i].Order < group[j].Order })
	order := make([]string, 0, len(group))
	for _, t := range group {
		if t.ID != srcID {
			order = append(order, t.ID)
		}
	}
	// Insert src just before the target (src takes target's slot; target shifts down).
	ti := 0
	for i, id := range order {
		if id == targetID {
			ti = i
			break
		}
	}
	order = append(order[:ti:ti], append([]string{srcID}, order[ti:]...)...)
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	var changed []domain.Task
	for _, t := range group {
		if np := pos[t.ID]; np != t.Order {
			t.Order = np
			changed = append(changed, t)
		}
	}
	return changed, true
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
