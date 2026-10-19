// SPDX-License-Identifier: MIT

package txntable

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"sort"
	"strings"
	"time"
)

// FollowUpItem is an ordered, preformatted task linked to a transaction.
type FollowUpItem struct {
	ID, Title, Due string
	Done           bool
	DueDate        time.Time
}

// FollowUpInfo combines task counts and their display order for a row.
type FollowUpInfo struct {
	Open, Total int
	Items       []FollowUpItem
}

// FollowUps groups linked tasks and orders open tasks before completed tasks,
// then dated tasks before undated tasks, then by due date. Ties remain stable.
func FollowUps(tasks []domain.Task, formatDue func(time.Time) string) map[string]FollowUpInfo {
	result := make(map[string]FollowUpInfo)
	for _, task := range tasks {
		if task.RelatedType != domain.RelatedTransaction || task.RelatedID == "" {
			continue
		}
		info := result[task.RelatedID]
		info.Total++
		done := task.Status == domain.StatusDone
		if !done {
			info.Open++
		}
		due := ""
		if !task.Due.IsZero() {
			due = formatDue(task.Due)
		}
		title := strings.TrimSpace(strings.TrimPrefix(task.Title, "Follow up:"))
		if title == "" {
			title = task.Title
		}
		info.Items = append(info.Items, FollowUpItem{ID: task.ID, Title: title, Done: done, Due: due, DueDate: task.Due})
		result[task.RelatedID] = info
	}
	for _, info := range result {
		sort.SliceStable(info.Items, func(i, j int) bool {
			a, b := info.Items[i], info.Items[j]
			if a.Done != b.Done {
				return !a.Done
			}
			if a.DueDate.IsZero() != b.DueDate.IsZero() {
				return !a.DueDate.IsZero()
			}
			return a.DueDate.Before(b.DueDate)
		})
	}
	return result
}
