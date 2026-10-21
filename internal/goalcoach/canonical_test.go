// SPDX-License-Identifier: MIT

package goalcoach

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func canonicalGoalFixture() (domain.Goal, time.Time) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	return domain.Goal{ID: "g", Name: "Trip", TargetAmount: money.New(10000, "EUR"), CurrentAmount: money.New(2000, "EUR"), MonthlyContribution: money.New(1000, "EUR"), TargetDate: now.AddDate(0, 2, 1)}, now
}

func TestReviewUsesCanonicalCoverageCurrencyAndCurrentMonth(t *testing.T) {
	g, now := canonicalGoalFixture()
	g.CurrentAmount = money.New(1000, "USD")
	g.MonthlyContribution = money.New(500, "USD")
	g.Allocations = []domain.GoalAllocation{{AccountID: "cash", Amount: money.New(500, "USD")}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	want := goals.EvaluateGoal(g, goals.TaskSummary{}, rates, now)
	r := ReviewGoal(g, goals.TaskSummary{}, rates, now)
	if r.Err != nil || r.Kind != KindBehind || !reflect.DeepEqual(r.Evaluation, want) || r.Monthly.Amount != 250 || r.Monthly.Currency != "EUR" || r.Extra.Currency != "EUR" || r.Extra.Amount != want.MonthlyNeeded.Amount-250 {
		t.Fatalf("review=%+v canonical=%+v", r, want)
	}
	finish, known, err := want.Project(g, r.Monthly, now)
	if err != nil || r.HasProjection != known || !r.ProjectedFinish.Equal(finish) {
		t.Fatalf("projection drift: %+v vs %v/%v/%v", r, finish, known, err)
	}
	// A deadline tomorrow still has a canonical current-month contribution slot.
	g.TargetDate = now.AddDate(0, 0, 1)
	r = ReviewGoal(g, goals.TaskSummary{}, rates, now)
	if r.Kind != KindBehind || r.Evaluation.MonthlyNeeded.Amount != 9250 {
		t.Fatalf("near deadline fell into old whole-month silence: %+v", r)
	}
}

func TestReviewPreservesUnknownWithoutMisclassifyingPlanOrCoverage(t *testing.T) {
	g, now := canonicalGoalFixture()
	rates := currency.Rates{Base: "EUR"}
	for _, tc := range []struct {
		name string
		g    domain.Goal
		want error
	}{
		{"saved FX", func() domain.Goal { v := g; v.CurrentAmount = money.New(1000, "USD"); return v }(), currency.ErrUnknownRate},
		{"plan FX", func() domain.Goal { v := g; v.MonthlyContribution = money.New(1000, "USD"); return v }(), currency.ErrUnknownRate},
		{"coverage overflow", func() domain.Goal {
			v := g
			v.TargetAmount = money.New(math.MaxInt64, "EUR")
			v.CurrentAmount = money.New(math.MaxInt64, "EUR")
			v.Allocations = []domain.GoalAllocation{{Amount: money.New(1, "EUR")}}
			return v
		}(), money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := ReviewGoal(tc.g, goals.TaskSummary{}, rates, now)
			if r.Kind != KindUnavailable || !errors.Is(r.Err, tc.want) {
				t.Fatalf("unknown became a verdict: %+v", r)
			}
		})
	}
	// Comparing/projecting a huge configured plan never multiplies it by months.
	g.MonthlyContribution = money.New(math.MaxInt64, "EUR")
	if r := ReviewGoal(g, goals.TaskSummary{}, rates, now); r.Err != nil || r.Kind != KindAhead || r.Monthly.Amount != math.MaxInt64 {
		t.Fatalf("valid huge plan wrapped: %+v", r)
	}
}

func TestReviewCurrentAmountAlreadyIncludesContributionHistory(t *testing.T) {
	g, now := canonicalGoalFixture()
	g.CurrentAmount = money.New(3000, "EUR")
	g.MonthlyContribution = money.Zero("EUR")
	g.Contributions = []domain.GoalContribution{{At: now.AddDate(0, -1, 0), Amount: money.New(1000, "EUR")}}
	g.ContributionHistory = []domain.GoalMonthTotal{{Month: "2026-08", Amount: money.New(1000, "EUR")}}
	r := ReviewGoal(g, goals.TaskSummary{}, currency.Rates{Base: "EUR"}, now)
	if r.Kind != KindUnfunded || r.Evaluation.Coverage.Saved.Amount != 3000 || r.Evaluation.Coverage.Remaining.Amount != 7000 || r.Evaluation.MonthlyNeeded.Amount != 2334 {
		t.Fatalf("history was counted twice or inferred as a monthly plan: %+v", r)
	}
	g.Allocations = []domain.GoalAllocation{{Amount: money.New(7000, "EUR")}}
	if r := ReviewGoal(g, goals.TaskSummary{}, currency.Rates{Base: "EUR"}, now); r.Kind != KindReached || r.Evaluation.Coverage.Saved.Amount != 3000 || r.Evaluation.Coverage.SetAside.Amount != 7000 {
		t.Fatalf("reserved coverage omitted: %+v", r)
	}
}

func TestReviewKeepsQuietPauseKindAndDeadlineBoundaries(t *testing.T) {
	g, now := canonicalGoalFixture()
	rates := currency.Rates{Base: "EUR"}
	for _, tc := range []struct {
		name string
		edit func(*domain.Goal)
		want Silence
	}{
		{"archived", func(v *domain.Goal) { v.Archived = true }, SilenceArchived},
		{"paused", func(v *domain.Goal) { v.PausedUntil = now.Add(time.Nanosecond) }, SilencePaused},
		{"recent", func(v *domain.Goal) { v.LastReviewedAt = now.Add(-quietPeriod + time.Nanosecond) }, SilenceRecent},
		{"no deadline", func(v *domain.Goal) { v.TargetDate = time.Time{} }, SilenceNoDeadline},
		{"deadline reached", func(v *domain.Goal) { v.TargetDate = now }, SilenceDeadline},
		{"habit", func(v *domain.Goal) { v.Kind = domain.GoalKindHabit }, SilenceNonfinancial},
		{"checklist", func(v *domain.Goal) { v.Kind = domain.GoalKindChecklist }, SilenceNonfinancial},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := g
			tc.edit(&v)
			r := ReviewGoal(v, goals.TaskSummary{}, rates, now)
			if r.Kind != KindSilent || r.Silence != tc.want {
				t.Fatalf("silence drift: %+v", r)
			}
		})
	}
	for _, edit := range []func(*domain.Goal){func(v *domain.Goal) { v.PausedUntil = now }, func(v *domain.Goal) { v.LastReviewedAt = now.Add(-quietPeriod) }} {
		v := g
		edit(&v)
		if r := ReviewGoal(v, goals.TaskSummary{}, rates, now); r.Kind == KindSilent {
			t.Fatalf("exact resume/quiet cutoff still silent: %+v", r)
		}
	}
}
