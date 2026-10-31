// SPDX-License-Identifier: MIT

package goals

import "github.com/monstercameron/CashFlux/internal/domain"

// TaskSummary contains checklist counts and at most three linked task previews.
// Open tasks precede completed tasks; each group preserves stored input order.
type TaskSummary struct {
	Done, Total int
	Preview     []domain.Task
}

// BuildTaskIndex summarizes all goal-linked tasks once without copying the full
// task collection. Unlinked tasks and blank related IDs never enter the index.
func BuildTaskIndex(tasks []domain.Task) map[string]TaskSummary {
	type bucket struct {
		done, total, openN, closedN int
		open, closed                [3]int
	}
	buckets := map[string]*bucket{}
	for i, t := range tasks {
		if t.RelatedType != domain.RelatedGoal || t.RelatedID == "" {
			continue
		}
		b := buckets[t.RelatedID]
		if b == nil {
			b = &bucket{}
			buckets[t.RelatedID] = b
		}
		b.total++
		if t.Status == domain.StatusDone {
			b.done++
			if b.closedN < 3 {
				b.closed[b.closedN] = i
				b.closedN++
			}
		} else if b.openN < 3 {
			b.open[b.openN] = i
			b.openN++
		}
	}
	out := make(map[string]TaskSummary, len(buckets))
	for id, b := range buckets {
		preview := make([]domain.Task, 0, min(3, b.total))
		for _, i := range b.open[:b.openN] {
			preview = append(preview, tasks[i])
		}
		for _, i := range b.closed[:min(b.closedN, 3-len(preview))] {
			preview = append(preview, tasks[i])
		}
		out[id] = TaskSummary{Done: b.done, Total: b.total, Preview: preview}
	}
	return out
}
