// SPDX-License-Identifier: MIT

package goals

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Progress is the kind-agnostic evaluated progress of a goal. It unifies the
// four goal kinds behind one shape so the UI can render any goal's headline the
// same way:
//
//   - financial: Percent from money saved/target; Done/Total unused.
//   - checklist: Done/Total are the completed/total linked to-dos; Percent = Done/Total.
//   - milestone: Percent is 0 or 100 from Goal.DoneAt; Done/Total are 0/1 then 1/1.
//   - habit:     Done/Total are check-ins/HabitTarget; Streak is the current run.
//
// Percent is always clamped to 0..100.
type Progress struct {
	Kind     domain.GoalKind
	Percent  int  // 0..100 (clamped)
	Done     int  // checklist/habit/milestone: completed count
	Total    int  // checklist/habit/milestone: total/target count
	Complete bool // the goal has reached its objective
	Streak   int  // habit only: current consecutive check-in streak
}

// LinkedTasks returns the to-dos linked to a goal (Task.RelatedType=RelatedGoal,
// RelatedID=goalID), preserving input order. A blank goalID matches nothing.
func LinkedTasks(tasks []domain.Task, goalID string) []domain.Task {
	if goalID == "" {
		return nil
	}
	var out []domain.Task
	for _, t := range tasks {
		if t.RelatedType == domain.RelatedGoal && t.RelatedID == goalID {
			out = append(out, t)
		}
	}
	return out
}

// TaskCounts returns how many of a goal's linked to-dos are done and how many
// are linked in total. It is the basis for a checklist goal's percent complete.
func TaskCounts(tasks []domain.Task, goalID string) (done, total int) {
	for _, t := range LinkedTasks(tasks, goalID) {
		total++
		if t.Status == domain.StatusDone {
			done++
		}
	}
	return done, total
}

// ChecklistPercent converts a done/total count into a 0..100 percentage. Zero
// total is 0% (an empty checklist has made no progress). The result is clamped.
func ChecklistPercent(done, total int) int {
	if total <= 0 {
		return 0
	}
	p := done * 100 / total
	switch {
	case p < 0:
		return 0
	case p > 100:
		return 100
	default:
		return p
	}
}

// cadenceDays is the approximate length of one cadence step in days, used to
// judge whether two habit check-ins are consecutive (within timing drift).
func cadenceDays(c domain.RecurringCadence) int {
	switch c {
	case domain.CadenceWeekly:
		return 7
	case domain.CadenceBiweekly:
		return 14
	case domain.CadenceSemimonthly:
		return 15
	case domain.CadenceQuarterly:
		return 91
	case domain.CadenceYearly:
		return 365
	default: // monthly and unknown
		return 30
	}
}

// HabitStreak returns the length of the current run of consecutive check-ins,
// counting back from the most recent. Two check-ins are consecutive when they
// are spaced no more than 1.5 cadence steps apart (so ordinary timing drift
// doesn't break the streak). An empty check-in list is a zero streak. `now` is
// unused today but kept in the signature so a future "streak lapsed because the
// latest check-in is stale" refinement doesn't change callers.
func HabitStreak(g domain.Goal, now time.Time) int {
	if len(g.CheckIns) == 0 {
		return 0
	}
	ins := make([]time.Time, len(g.CheckIns))
	copy(ins, g.CheckIns)
	sort.Slice(ins, func(i, j int) bool { return ins[i].After(ins[j]) }) // newest first
	tol := time.Duration(cadenceDays(g.HabitCadence)) * 36 * time.Hour   // 1.5 days-per-step
	streak := 1
	for i := 1; i < len(ins); i++ {
		if ins[i-1].Sub(ins[i]) <= tol {
			streak++
		} else {
			break
		}
	}
	return streak
}

// EvaluateProgress computes the unified Progress for a goal of any kind. `tasks`
// is the full task list (LinkedTasks filters it for checklist goals); `now` is
// the reference time for habit streaks. Financial progress reuses Percent /
// IsComplete so its clamping and overfund handling match the rest of the package.
func EvaluateProgress(g domain.Goal, tasks []domain.Task, now time.Time) Progress {
	switch g.EffectiveKind() {
	case domain.GoalKindChecklist:
		done, total := TaskCounts(tasks, g.ID)
		return Progress{
			Kind:     domain.GoalKindChecklist,
			Percent:  ChecklistPercent(done, total),
			Done:     done,
			Total:    total,
			Complete: total > 0 && done >= total,
		}
	case domain.GoalKindMilestone:
		done := g.IsMilestoneDone()
		p := Progress{Kind: domain.GoalKindMilestone, Total: 1, Complete: done}
		if done {
			p.Percent, p.Done = 100, 1
		}
		return p
	case domain.GoalKindHabit:
		done := len(g.CheckIns)
		total := g.HabitTarget
		return Progress{
			Kind:     domain.GoalKindHabit,
			Percent:  ChecklistPercent(done, total),
			Done:     done,
			Total:    total,
			Complete: total > 0 && done >= total,
			Streak:   HabitStreak(g, now),
		}
	default: // financial (and the legacy empty kind)
		complete, _ := IsComplete(g)
		return Progress{
			Kind:     domain.GoalKindFinancial,
			Percent:  Percent(g),
			Complete: complete,
		}
	}
}
