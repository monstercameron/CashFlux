// SPDX-License-Identifier: MIT

package goals

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// GoalState is a goal's coarse lifecycle bucket, used by the dashboard summary and
// the card badge. It is timing + funding, independent of the Archived flag (an
// archived goal still has a state — usually Completed).
type GoalState string

const (
	// StateCurrent is an active goal still working toward its objective, not overdue.
	StateCurrent GoalState = "current"
	// StateMissed is a dated goal whose target date has passed while still short of
	// its objective, and which is not paused — the deadline was missed.
	StateMissed GoalState = "missed"
	// StateCompleted is a goal that has reached its objective (see Reached).
	StateCompleted GoalState = "completed"
)

// Reached reports whether a goal has met its objective. This is the FIRST-CLASS
// completion definition: a financial goal is reached when it is fully FUNDED —
// committed savings PLUS virtual earmarks cover the target (CoverageMinor) — so
// grounding a goal in real, set-aside money counts, not only money that has moved.
// Non-financial goals defer to their kind's completion (checklist done, milestone
// marked, habit target hit).
func Reached(g domain.Goal, tasks []domain.Task, now time.Time) bool {
	if g.IsFinancial() {
		return g.TargetAmount.Amount > 0 && CoverageMinor(g) >= g.TargetAmount.Amount
	}
	return EvaluateProgress(g, tasks, now).Complete
}

// Classify buckets a goal into Current / Missed / Completed at reference time now.
// Precedence: reached → completed; else a passed, unpaused target date → missed;
// otherwise current. The Archived flag is intentionally NOT consulted so callers
// can decide whether to fold archived goals into the completed count.
func Classify(g domain.Goal, tasks []domain.Task, now time.Time) GoalState {
	if Reached(g, tasks, now) {
		return StateCompleted
	}
	if !g.TargetDate.IsZero() && !g.TargetDate.After(now) && !g.IsPaused(now) {
		return StateMissed
	}
	return StateCurrent
}

// StateCounts is a tally of goals by lifecycle state — the dashboard widget's data.
type StateCounts struct {
	Current   int
	Missed    int
	Completed int
}

// Total is the sum across all three buckets.
func (c StateCounts) Total() int { return c.Current + c.Missed + c.Completed }

// CountByState tallies the goals by Classify. Sinking funds are excluded (they are
// ongoing buckets, not one-off objectives with a finish line). When
// includeArchived is false, archived goals are skipped entirely — pass true for a
// lifetime "completed" count that includes goals moved to the Achieved section.
func CountByState(goals []domain.Goal, tasks []domain.Task, now time.Time, includeArchived bool) StateCounts {
	var c StateCounts
	for _, g := range goals {
		if g.IsSinkingFund {
			continue
		}
		if g.Archived && !includeArchived {
			continue
		}
		switch Classify(g, tasks, now) {
		case StateMissed:
			c.Missed++
		case StateCompleted:
			c.Completed++
		default:
			c.Current++
		}
	}
	return c
}
