// SPDX-License-Identifier: MIT

package dashboardview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
)

// EvaluatedGoal keeps the original stored goal beside its canonical read model.
// Callers retain Goal for callbacks and never persist Evaluation's derived money.
type EvaluatedGoal struct {
	Goal       domain.Goal
	Evaluation goals.Evaluation
}

// GoalFacts contains active dashboard goals and the lifecycle tally. Unavailable
// counts financial goals whose state cannot be established from available FX.
type GoalFacts struct {
	Rows        []EvaluatedGoal
	Counts      goals.StateCounts
	Unavailable int
}

// DeriveGoals evaluates each active goal once using one shared linked-task index.
// Ongoing sinking funds remain eligible for the featured tile but not the tally.
func DeriveGoals(input []domain.Goal, tasks []domain.Task, rates currency.Rates, now time.Time) GoalFacts {
	index := goals.BuildTaskIndex(tasks)
	var out GoalFacts
	for _, goal := range input {
		if goal.Archived {
			continue
		}
		evaluation := goals.EvaluateGoal(goal, index[goal.ID], rates, now)
		out.Rows = append(out.Rows, EvaluatedGoal{Goal: goal, Evaluation: evaluation})
		if goal.IsSinkingFund {
			continue
		}
		if evaluation.Err != nil {
			out.Unavailable++
		}
		switch evaluation.State {
		case goals.StateCompleted:
			out.Counts.Completed++
		case goals.StateMissed:
			out.Counts.Missed++
		default:
			out.Counts.Current++
		}
	}
	return out
}

// Featured chooses the first active goal, or the known goal closest to completion
// when requested. Original order breaks ties. Unknown coverage never wins a rank
// by interpreting a foreign-currency amount as if it were in the target currency.
func (facts GoalFacts) Featured(byProgress bool) (EvaluatedGoal, bool) {
	if len(facts.Rows) == 0 {
		return EvaluatedGoal{}, false
	}
	chosen := facts.Rows[0]
	if !byProgress {
		return chosen, true
	}
	progress := func(row EvaluatedGoal) int {
		if row.Evaluation.Err != nil {
			return -1
		}
		if row.Goal.IsFinancial() {
			return row.Evaluation.Coverage.CoveragePercent
		}
		return row.Evaluation.Progress.Percent
	}
	best := progress(chosen)
	for _, row := range facts.Rows[1:] {
		if percent := progress(row); percent > best {
			chosen, best = row, percent
		}
	}
	return chosen, true
}
