// SPDX-License-Identifier: MIT

package goalview

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
)

// OrderedOpen returns a sorted copy of all visible, non-archived goals. Explicit
// toolbar sorts span the default lifecycle and funding-health sections.
func OrderedOpen(v View, key string, now time.Time) []domain.Goal {
	var out []domain.Goal
	for _, g := range v.All {
		if !g.Archived {
			out = append(out, g)
		}
	}
	Sort(out, key, v.Tasks, now)
	return out
}

// Sort orders a caller-owned goal slice by the toolbar sort key. Ties retain
// the existing actionable order. It never mutates tasks or goal children.
func Sort(gs []domain.Goal, key string, tasks []domain.Task, now time.Time) {
	// Decorate only the requested rank. Sorting never scans the task list or
	// sorts habit check-ins inside its comparison function.
	percent, counts := map[string]int{}, map[string]int{}
	if key == "closest" || key == "farthest" || key == "complexity" {
		done := map[string]int{}
		for _, task := range tasks {
			if task.RelatedType != domain.RelatedGoal || task.RelatedID == "" {
				continue
			}
			counts[task.RelatedID]++
			if task.Status == domain.StatusDone {
				done[task.RelatedID]++
			}
		}
		for _, g := range gs {
			if key == "complexity" {
				continue
			}
			if g.EffectiveKind() == domain.GoalKindChecklist {
				percent[g.ID] = goals.ChecklistPercent(done[g.ID], counts[g.ID])
			} else {
				percent[g.ID] = goals.EvaluateProgress(g, nil, now).Percent
			}
		}
	}
	pct := func(g domain.Goal) int { return percent[g.ID] }
	steps := func(g domain.Goal) int { return counts[g.ID] }
	switch key {
	case "closest":
		sort.SliceStable(gs, func(i, j int) bool {
			if pi, pj := pct(gs[i]), pct(gs[j]); pi != pj {
				return pi > pj // nearly-there first
			}
			return goals.LessForList(gs[i], gs[j])
		})
	case "farthest":
		sort.SliceStable(gs, func(i, j int) bool {
			if pi, pj := pct(gs[i]), pct(gs[j]); pi != pj {
				return pi < pj // just-getting-started first
			}
			return goals.LessForList(gs[i], gs[j])
		})
	case "complexity":
		sort.SliceStable(gs, func(i, j int) bool {
			if si, sj := steps(gs[i]), steps(gs[j]); si != sj {
				return si > sj // most steps to work through first
			}
			return goals.LessForList(gs[i], gs[j])
		})
	case "deadline":
		sort.SliceStable(gs, func(i, j int) bool {
			ai, aj := gs[i].TargetDate.IsZero(), gs[j].TargetDate.IsZero()
			if ai != aj {
				return !ai // dated goals before undated
			}
			if !ai && !aj && !gs[i].TargetDate.Equal(gs[j].TargetDate) {
				return gs[i].TargetDate.Before(gs[j].TargetDate)
			}
			return gs[i].Name < gs[j].Name
		})
	case "name":
		sort.SliceStable(gs, func(i, j int) bool { return gs[i].Name < gs[j].Name })
	case "priority":
		sort.SliceStable(gs, func(i, j int) bool {
			if pi, pj := gs[i].PriorityRank(), gs[j].PriorityRank(); pi != pj {
				return pi < pj // high (1) first; unprioritized last
			}
			return goals.LessForList(gs[i], gs[j])
		})
	default: // GoalSortActionable
		sort.SliceStable(gs, func(i, j int) bool { return goals.LessForList(gs[i], gs[j]) })
	}
}
