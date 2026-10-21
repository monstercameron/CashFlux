// SPDX-License-Identifier: MIT

// Package taskview prepares the shared task filters, counts and calendar buckets
// used by the to-do screen without depending on a browser or UI framework.
package taskview

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasksort"
)

// Counts describes all tasks independently of the current view filters.
type Counts struct {
	Open, Overdue, Today, Done, Total, DonePct int
}

// Count tallies task completion and open due dates using calendar dates, so an
// unfinished task due earlier today is not treated as overdue.
func Count(tasks []domain.Task, todayISO string) Counts {
	var c Counts
	for _, t := range tasks {
		c.Total++
		if t.Status == domain.StatusDone {
			c.Done++
			continue
		}
		c.Open++
		if t.Due.IsZero() || todayISO == "" {
			continue
		}
		switch due := dateutil.FormatDate(t.Due); {
		case due < todayISO:
			c.Overdue++
		case due == todayISO:
			c.Today++
		}
	}
	if c.Total > 0 {
		c.DonePct = c.Done * 100 / c.Total
	}
	return c
}

// Filter configures the same flat task set for list, board and calendar views.
// Empty Link means every link type; "none" means tasks with no complete link.
type Filter struct {
	HideDone             bool
	Priority             domain.TaskPriority
	Search, Link, LinkID string
	Quick                tasksort.QuickView
	TodayISO             string
}

// Select filters in a single pass before hierarchy and pagination. It preserves
// storage order, never mutates input, and lets matching children survive when a
// parent is excluded. Search matches titles and notes case-insensitively.
func Select(tasks []domain.Task, f Filter) []domain.Task {
	q := strings.ToLower(strings.TrimSpace(f.Search))
	quick := tasksort.ParseQuickView(string(f.Quick))
	out := make([]domain.Task, 0, len(tasks))
	for _, t := range tasks {
		if f.HideDone && t.Status == domain.StatusDone || f.Priority.Valid() && t.Priority != f.Priority {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.Title), q) && !strings.Contains(strings.ToLower(t.Notes), q) {
			continue
		}
		linked := t.RelatedID != "" && t.RelatedType != "" && t.RelatedType != domain.RelatedNone
		if f.Link != "" {
			if f.Link == "none" {
				if linked {
					continue
				}
			} else if !linked || string(t.RelatedType) != f.Link {
				continue
			}
		}
		if f.LinkID != "" && t.RelatedID != f.LinkID {
			continue
		}
		if quick != tasksort.QuickAll && f.TodayISO != "" {
			if t.Status == domain.StatusDone || t.Due.IsZero() {
				continue
			}
			due := dateutil.FormatDate(t.Due)
			if quick == tasksort.QuickToday && due != f.TodayISO || quick == tasksort.QuickOverdue && due >= f.TodayISO {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// ByDueDay buckets dated tasks by their own calendar date, ordering each day by
// high/medium/low priority and then case-sensitive title, preserving ties.
func ByDueDay(tasks []domain.Task) map[string][]domain.Task {
	out := make(map[string][]domain.Task)
	for _, t := range tasks {
		if !t.Due.IsZero() {
			key := dateutil.FormatDate(t.Due)
			out[key] = append(out[key], t)
		}
	}
	rank := func(p domain.TaskPriority) int {
		switch p {
		case domain.PriorityHigh:
			return 0
		case domain.PriorityLow:
			return 2
		default:
			return 1
		}
	}
	for _, day := range out {
		sort.SliceStable(day, func(i, j int) bool {
			if a, b := rank(day[i].Priority), rank(day[j].Priority); a != b {
				return a < b
			}
			return day[i].Title < day[j].Title
		})
	}
	return out
}
