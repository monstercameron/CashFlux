// SPDX-License-Identifier: MIT

package goalcoach

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

// KindUnavailable preserves an unknown calculation as a coaching result rather
// than turning missing FX or arithmetic failure into a quiet/on-track verdict.
const KindUnavailable Kind = KindReached + 1

// Silence identifies the existing deliberate omissions from savings coaching.
type Silence string

const (
	// SilenceArchived excludes a goal no longer being pursued.
	SilenceArchived Silence = "archived"
	// SilencePaused respects the household's explicit pause.
	SilencePaused Silence = "paused"
	// SilenceRecent avoids repeating a review within the fourteen-day quiet period.
	SilenceRecent Silence = "reviewed within fourteen days"
	// SilenceNonfinancial directs kind-specific progress questions to list_goals.
	SilenceNonfinancial Silence = "nonfinancial goal; use list_goals for progress"
	// SilenceNoDeadline does not infer a commitment without a target date.
	SilenceNoDeadline Silence = "no target date"
	// SilenceDeadline does not propose future monthly payments for a past deadline.
	SilenceDeadline Silence = "deadline reached; no future monthly adjustment inferred"
)

// Review is one stored goal's canonical coverage and configured-plan comparison.
// Money stays in the target currency. Contributions and ContributionHistory are
// already represented by CurrentAmount and are never added to it again. This is
// a comparison of the configured monthly plan, not an observed contribution pace.
type Review struct {
	Target           money.Money
	GoalID, GoalName string
	Kind             Kind
	Silence          Silence
	Evaluation       goals.Evaluation
	Monthly          money.Money
	HasMonthlyPlan   bool
	Extra            money.Money
	ProjectedFinish  time.Time
	HasProjection    bool
	Err              error
}

// ReviewGoal retains the coach's quiet policy while delegating all financial
// coverage, required contribution and projection calculations to goals.Evaluation.
func ReviewGoal(g domain.Goal, tasks goals.TaskSummary, rates currency.Rates, now time.Time) Review {
	r := Review{GoalID: g.ID, GoalName: g.Name, Target: g.TargetAmount}
	switch {
	case g.Archived:
		r.Silence = SilenceArchived
		return r
	case g.IsPaused(now):
		r.Silence = SilencePaused
		return r
	case !g.LastReviewedAt.IsZero() && now.Sub(g.LastReviewedAt) < quietPeriod:
		r.Silence = SilenceRecent
		return r
	case !g.IsFinancial():
		r.Silence = SilenceNonfinancial
		return r
	}
	r.Evaluation = goals.EvaluateGoal(g, tasks, rates, now)
	if r.Evaluation.Err != nil {
		r.Kind, r.Err = KindUnavailable, r.Evaluation.Err
		return r
	}
	if r.Evaluation.Complete {
		r.Kind = KindReached
		return r
	}
	if g.TargetDate.IsZero() {
		r.Silence = SilenceNoDeadline
		return r
	}
	if !g.TargetDate.After(now) || !r.Evaluation.HasMonthlyNeeded {
		r.Silence = SilenceDeadline
		return r
	}
	if g.MonthlyContribution.Amount <= 0 {
		r.Kind = KindUnfunded
		return r
	}
	var ok bool
	r.Monthly, ok, r.Err = r.Evaluation.Assignment(g, rates)
	if r.Err != nil {
		r.Kind = KindUnavailable
		return r
	}
	r.HasMonthlyPlan = ok
	if !ok {
		r.Kind = KindUnfunded
		return r
	}
	r.ProjectedFinish, r.HasProjection, r.Err = r.Evaluation.Project(g, r.Monthly, now)
	if r.Err != nil {
		r.Kind = KindUnavailable
		return r
	}
	if r.Monthly.Amount < r.Evaluation.MonthlyNeeded.Amount {
		r.Kind = KindBehind
		r.Extra, r.Err = r.Evaluation.MonthlyNeeded.Sub(r.Monthly)
		if r.Err != nil {
			r.Kind = KindUnavailable
		}
		return r
	}
	r.Kind = KindOnTrack
	if r.HasProjection && r.ProjectedFinish.Before(g.TargetDate) {
		r.Kind = KindAhead
	}
	return r
}

// ReviewAll prepares one task index and retains silent results so a caller can
// explain exclusions without claiming unreviewed goals are financially on track.
// Different currencies are not ranked against each other's raw minor amounts.
func ReviewAll(all []domain.Goal, tasks []domain.Task, rates currency.Rates, now time.Time) []Review {
	index := goals.BuildTaskIndex(tasks)
	out := make([]Review, 0, len(all))
	for _, g := range all {
		out = append(out, ReviewGoal(g, index[g.ID], rates, now))
	}
	rank := func(k Kind) int {
		switch k {
		case KindUnavailable:
			return 0
		case KindBehind:
			return 1
		case KindUnfunded:
			return 2
		case KindReached:
			return 3
		case KindAhead:
			return 4
		case KindOnTrack:
			return 5
		default:
			return 6
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if rank(a.Kind) != rank(b.Kind) {
			return rank(a.Kind) < rank(b.Kind)
		}
		if a.GoalName != b.GoalName {
			return a.GoalName < b.GoalName
		}
		return a.GoalID < b.GoalID
	})
	return out
}
