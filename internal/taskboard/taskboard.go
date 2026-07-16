// SPDX-License-Identifier: MIT

// Package taskboard groups a flat list of to-do tasks into ordered kanban
// columns for the board view. It is pure (no syscall/js): all grouping,
// ordering, and "advance to next column" logic lives here and is unit-tested on
// native Go, so the wasm/UI layer stays a thin shell over it.
package taskboard

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// GroupBy selects how Columns buckets the tasks.
type GroupBy int

const (
	// GroupByStatus groups tasks by their completion status (To do, Done).
	GroupByStatus GroupBy = iota
	// GroupByPriority groups tasks by priority, highest first (High, Medium, Low).
	GroupByPriority
)

// Column is one kanban lane: a stable Key (the raw domain enum value the
// coordinator maps back to a status/priority when OnMove fires), a Title key for
// i18n lookup, and the tasks that fall in it, pre-sorted for display.
type Column struct {
	// Key is the raw enum value for this column — a domain.TaskStatus string
	// ("open"/"done") for status grouping, or a domain.TaskPriority string
	// ("high"/"med"/"low") for priority grouping. It is what OnMove passes back
	// so the caller can set the moved task's field without re-deriving it.
	Key string
	// Title is an i18n message key (e.g. "todoboard.colTodo") for the column
	// header label, so the UI resolves it through uistate.T.
	Title string
	// Tasks are the column's tasks, sorted by Due (dated ascending, undated
	// last) then Title, stably.
	Tasks []domain.Task
}

// statusOrder is the fixed left-to-right column order for status grouping.
// Done is kept last. The domain models only open and done; a task with an empty
// or unrecognized status falls back to the "To do" (open) lane so nothing is
// dropped from the board.
var statusOrder = []struct {
	key   domain.TaskStatus
	title string
}{
	{domain.StatusOpen, "todoboard.colTodo"},
	{domain.StatusDone, "todoboard.colDone"},
}

// priorityOrder is the fixed left-to-right column order for priority grouping,
// highest priority first. A task with an empty or unrecognized priority falls
// back to the Medium lane (the app's default priority) so nothing is dropped.
var priorityOrder = []struct {
	key   domain.TaskPriority
	title string
}{
	{domain.PriorityHigh, "todoboard.colHigh"},
	{domain.PriorityMedium, "todoboard.colMedium"},
	{domain.PriorityLow, "todoboard.colLow"},
}

// Columns groups tasks into ordered board columns by the given dimension. Every
// task lands in exactly one column (unrecognized status/priority values fall
// back to To do / Medium respectively), so the board never silently drops work.
// Within each column tasks are sorted by Due (dated ascending, then undated) and
// then Title, stably, so equal keys keep their input order.
func Columns(tasks []domain.Task, by GroupBy) []Column {
	switch by {
	case GroupByPriority:
		return groupByPriority(tasks)
	default:
		return groupByStatus(tasks)
	}
}

func groupByStatus(tasks []domain.Task) []Column {
	buckets := make(map[domain.TaskStatus][]domain.Task, len(statusOrder))
	for _, t := range tasks {
		buckets[normalizeStatus(t.Status)] = append(buckets[normalizeStatus(t.Status)], t)
	}
	cols := make([]Column, 0, len(statusOrder))
	for _, o := range statusOrder {
		col := Column{Key: string(o.key), Title: o.title, Tasks: buckets[o.key]}
		sortColumn(col.Tasks)
		cols = append(cols, col)
	}
	return cols
}

func groupByPriority(tasks []domain.Task) []Column {
	buckets := make(map[domain.TaskPriority][]domain.Task, len(priorityOrder))
	for _, t := range tasks {
		buckets[normalizePriority(t.Priority)] = append(buckets[normalizePriority(t.Priority)], t)
	}
	cols := make([]Column, 0, len(priorityOrder))
	for _, o := range priorityOrder {
		col := Column{Key: string(o.key), Title: o.title, Tasks: buckets[o.key]}
		sortColumn(col.Tasks)
		cols = append(cols, col)
	}
	return cols
}

// normalizeStatus maps a task's stored status to a known column key, folding the
// empty/unrecognized zero value into StatusOpen so it shows in the To do lane.
func normalizeStatus(s domain.TaskStatus) domain.TaskStatus {
	if s == domain.StatusDone {
		return domain.StatusDone
	}
	return domain.StatusOpen
}

// normalizePriority maps a task's stored priority to a known column key, folding
// the empty/unrecognized zero value into PriorityMedium (the app default).
func normalizePriority(p domain.TaskPriority) domain.TaskPriority {
	switch p {
	case domain.PriorityHigh, domain.PriorityMedium, domain.PriorityLow:
		return p
	default:
		return domain.PriorityMedium
	}
}

// sortColumn orders a column's tasks in place: dated tasks first, ascending by
// due date; undated tasks after; ties (and undated tasks) broken by Title. The
// sort is stable so tasks equal on both keys keep their input order.
func sortColumn(tasks []domain.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		aZero, bZero := a.Due.IsZero(), b.Due.IsZero()
		if aZero != bZero {
			return !aZero // a dated task sorts before an undated one
		}
		if !aZero && !bZero && !a.Due.Equal(b.Due) {
			return a.Due.Before(b.Due)
		}
		return a.Title < b.Title
	})
}

// NextKey returns the key of the column immediately to the right of currentKey
// for the given grouping — the target the board's one-click "Next" affordance
// advances a card to. ok is false when currentKey is the last (rightmost) column
// or is not a recognized key, in which case the card has nowhere to advance.
func NextKey(by GroupBy, currentKey string) (nextKey string, ok bool) {
	switch by {
	case GroupByPriority:
		for i, o := range priorityOrder {
			if string(o.key) == currentKey {
				if i+1 < len(priorityOrder) {
					return string(priorityOrder[i+1].key), true
				}
				return "", false
			}
		}
	default:
		for i, o := range statusOrder {
			if string(o.key) == currentKey {
				if i+1 < len(statusOrder) {
					return string(statusOrder[i+1].key), true
				}
				return "", false
			}
		}
	}
	return "", false
}
