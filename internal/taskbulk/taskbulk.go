// SPDX-License-Identifier: MIT

// Package taskbulk computes what a bulk edit over a selection of to-dos would
// change, without applying it. The to-do list had no selection mechanism at all
// (C402): reassigning six tasks to one person, or pushing a week of overdue
// items out, meant opening six edit modals. The gesture is the easy half; the
// hard half is deciding precisely what a bulk edit means, and that decision
// belongs in a pure package with tests rather than inside a click handler.
//
// Two rules shape everything here:
//
//   - A plan returns only the tasks that actually CHANGE. Writing a row whose
//     fields already match is not a no-op — it bumps its revision, appears in
//     the undo snapshot, and inflates the "12 tasks updated" toast into a lie.
//   - Completion is not part of a plan. Completing a recurring task spawns its
//     successor, which is a store-level transaction, so the caller drives that
//     through appstate.CompleteTask. Plan covers only field edits.
package taskbulk

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// DueShift names where a bulk edit moves the due date.
//
// The targets are ABSOLUTE (today, tomorrow, next week, next month) rather than
// relative offsets applied to each task's own date, because the point of a bulk
// reschedule is to land a scattered pile on one day. Applying "+7 days" to a
// mixed selection just produces a differently-scattered pile. PushWeek is the
// one relative option, for the "these are all a week behind" case where keeping
// their relative order matters.
type DueShift string

const (
	// ShiftNone leaves the due date alone.
	ShiftNone DueShift = ""
	// ShiftToday moves the due date to today.
	ShiftToday DueShift = "today"
	// ShiftTomorrow moves the due date to tomorrow.
	ShiftTomorrow DueShift = "tomorrow"
	// ShiftNextWeek moves the due date to a week from today.
	ShiftNextWeek DueShift = "next-week"
	// ShiftNextMonth moves the due date to a month from today.
	ShiftNextMonth DueShift = "next-month"
	// ShiftPushWeek adds seven days to each task's OWN due date, preserving the
	// order of a set that has slipped together. A task with no due date falls
	// back to a week from today — there is nothing to push otherwise.
	ShiftPushWeek DueShift = "push-week"
	// ShiftClear removes the due date.
	ShiftClear DueShift = "clear"
)

// Unassigned is the MemberID sentinel meaning "take the owner off these tasks".
// An empty MemberID cannot mean that, because empty already means "leave the
// owner alone" — without a distinct sentinel there is no way to bulk-unassign.
const Unassigned = "-"

// Edit is one bulk field change. A zero Edit changes nothing.
type Edit struct {
	// MemberID assigns an owner. Empty leaves the owner untouched; Unassigned
	// clears it.
	MemberID string
	// Due moves the due date. ShiftNone leaves it untouched.
	Due DueShift
}

// Empty reports whether the edit would change nothing regardless of selection.
func (e Edit) Empty() bool { return e.MemberID == "" && e.Due == ShiftNone }

// DueAfter reports the due date a shift produces for task t at time now, and
// whether the task ends up with a due date at all. Dates land on midnight in
// now's location: a to-do is due on a day, not at a moment, and carrying a
// stale time-of-day through a reschedule makes "due tomorrow" sort against
// tasks due the same day in an order nobody chose.
func DueAfter(t domain.Task, s DueShift, now time.Time) (time.Time, bool) {
	midnight := func(x time.Time) time.Time {
		return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, now.Location())
	}
	switch s {
	case ShiftToday:
		return midnight(now), true
	case ShiftTomorrow:
		return midnight(now).AddDate(0, 0, 1), true
	case ShiftNextWeek:
		return midnight(now).AddDate(0, 0, 7), true
	case ShiftNextMonth:
		return midnight(now).AddDate(0, 1, 0), true
	case ShiftPushWeek:
		if t.Due.IsZero() {
			return midnight(now).AddDate(0, 0, 7), true
		}
		return midnight(t.Due).AddDate(0, 0, 7), true
	case ShiftClear:
		return time.Time{}, false
	}
	return t.Due, !t.Due.IsZero()
}

// Plan returns the selected tasks that the edit would actually change, each
// already carrying its new field values, in the order they appear in tasks.
// Tasks that are not selected, or that the edit would leave identical, are
// omitted — see the package comment.
func Plan(tasks []domain.Task, selected map[string]bool, e Edit, now time.Time) []domain.Task {
	if e.Empty() || len(selected) == 0 {
		return nil
	}
	var out []domain.Task
	for _, t := range tasks {
		if !selected[t.ID] {
			continue
		}
		next := t
		if e.MemberID == Unassigned {
			next.MemberID = ""
		} else if e.MemberID != "" {
			next.MemberID = e.MemberID
		}
		if e.Due != ShiftNone {
			due, has := DueAfter(t, e.Due, now)
			if has {
				next.Due = due
			} else {
				next.Due = time.Time{}
			}
		}
		if next.MemberID == t.MemberID && next.Due.Equal(t.Due) {
			continue
		}
		out = append(out, next)
	}
	return out
}

// Completable returns the ids of the selected tasks that are still open, in the
// order they appear in tasks. Bulk-completing a selection that includes already
// done tasks must not re-complete them: a second completion on a recurring task
// would spawn a second successor.
func Completable(tasks []domain.Task, selected map[string]bool) []string {
	if len(selected) == 0 {
		return nil
	}
	var out []string
	for _, t := range tasks {
		if selected[t.ID] && t.Status == domain.StatusOpen {
			out = append(out, t.ID)
		}
	}
	return out
}

// Deletable returns the ids of the selected tasks, in the order they appear in
// tasks, skipping ids in the selection that no longer exist. A selection can
// outlive the rows it names — another tab, a rule, or a completed recurrence
// can remove one between the click and the confirm.
func Deletable(tasks []domain.Task, selected map[string]bool) []string {
	if len(selected) == 0 {
		return nil
	}
	var out []string
	for _, t := range tasks {
		if selected[t.ID] {
			out = append(out, t.ID)
		}
	}
	return out
}

// Range returns every id in order between a and b inclusive, for shift-click
// range selection. It returns nil when either anchor is missing from order,
// rather than guessing at a partial range.
func Range(order []string, a, b string) []string {
	ai, bi := -1, -1
	for i, id := range order {
		if id == a {
			ai = i
		}
		if id == b {
			bi = i
		}
	}
	if ai < 0 || bi < 0 {
		return nil
	}
	if ai > bi {
		ai, bi = bi, ai
	}
	return order[ai : bi+1]
}
