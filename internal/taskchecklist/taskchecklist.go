// SPDX-License-Identifier: MIT

// Package taskchecklist instantiates checklist templates — a parent task plus
// its ordered sub-tasks (Task.ParentID nesting) — for recurring financial
// rituals like the month-end close and tax preparation. Titles arrive already
// localized from the caller; this package owns only the structure and dates.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package taskchecklist

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Item is one step of a checklist template.
type Item struct {
	// Title is the localized step title.
	Title string
	// DueOffsetDays shifts this step's due date relative to the checklist's
	// due date (0 = same day; negative = earlier).
	DueOffsetDays int
}

// Instantiate builds a parent checklist task plus one open sub-task per item,
// in order. The parent carries the checklist title and the due date; each
// child nests under it (ParentID) with Order preserving the given sequence so
// the manual sort shows the steps as written. newID mints ids (one per task).
func Instantiate(title string, items []Item, due time.Time, newID func() string) []domain.Task {
	parent := domain.Task{
		ID: newID(), Title: title, Due: due,
		Status: domain.StatusOpen, Priority: domain.PriorityMedium,
		Source: domain.SourceManual,
	}
	out := make([]domain.Task, 0, len(items)+1)
	out = append(out, parent)
	for i, it := range items {
		out = append(out, domain.Task{
			ID: newID(), Title: it.Title, ParentID: parent.ID,
			Due:    due.AddDate(0, 0, it.DueOffsetDays),
			Status: domain.StatusOpen, Priority: domain.PriorityMedium,
			Source: domain.SourceManual, Order: i + 1,
		})
	}
	return out
}
