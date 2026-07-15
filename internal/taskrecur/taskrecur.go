// SPDX-License-Identifier: MIT

// Package taskrecur contains the pure logic for recurring task auto-spawning.
// It has no syscall/js dependency and is safe to unit-test on native Go.
package taskrecur

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// NextOccurrence returns the next open occurrence of a completed recurring task.
// It returns (task, true) when done.Recurrence is non-empty, or (zero, false) for
// a one-shot task.
//
// The new task carries the same Title, Notes, Priority, ParentID, RelatedType,
// RelatedID, MemberID, and Recurrence as the original; Status is StatusOpen and
// Source mirrors the original task's Source. The new ID is the caller-supplied
// newID (keeping ID generation outside pure logic so the package stays testable
// and deterministic).
//
// Due advancement base: if done.Due is set the next Due is done.Recurrence.Next(done.Due),
// preserving the original calendar anchor even when the task is completed late.
// If done.Due is zero (no due date was ever set) now is used as the base instead,
// so the next occurrence gets a sensible near-future anchor rather than the zero
// time.
func NextOccurrence(done domain.Task, newID string, now time.Time) (domain.Task, bool) {
	if done.Recurrence == "" {
		return domain.Task{}, false
	}

	base := done.Due
	if base.IsZero() {
		base = now
	}

	next := domain.Task{
		ID:          newID,
		Title:       done.Title,
		Notes:       done.Notes,
		Due:         done.Recurrence.Next(base),
		Status:      domain.StatusOpen,
		Priority:    done.Priority,
		ParentID:    done.ParentID,
		RelatedType: done.RelatedType,
		RelatedID:   done.RelatedID,
		MemberID:    done.MemberID,
		Source:      done.Source,
		Recurrence:  done.Recurrence,
		// The reminder lead is a property of the recurring series, so each spawned
		// occurrence inherits it and surfaces its own reminder window.
		ReminderLeadDays: done.ReminderLeadDays,
	}
	return next, true
}

// ReminderDue reports whether an open task's in-app reminder window has opened as
// of now — i.e. it is not done, has a Due date, and (Due − ReminderLeadDays) is on
// or before now. A ReminderLeadDays of 0 means the window opens on the due date
// itself; a positive lead opens it that many days earlier. Tasks with no Due date
// are never reminder-due (there is nothing to count back from).
//
// This is the pure predicate the needs-attention digest uses to surface tasks
// whose deadline is approaching, before they tip into overdue.
func ReminderDue(t domain.Task, now time.Time) bool {
	if t.Status != domain.StatusOpen || t.Due.IsZero() {
		return false
	}
	window := t.Due.AddDate(0, 0, -t.ReminderLeadDays)
	return !window.After(now)
}
