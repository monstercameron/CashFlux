// SPDX-License-Identifier: MIT

package goals

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEvaluateCoverageForeignReservationsAndParity(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, foreign := range []bool{false, true} {
		g := mkGoalWithAlloc("g", 10000, 2000, alloc("a", 1000))
		if foreign {
			g.Allocations[0].Amount = money.New(500, "EUR")
		}
		g.TargetDate = now.AddDate(0, 2, 0)
		before := append([]domain.GoalAllocation(nil), g.Allocations...)
		e := EvaluateGoal(g, TaskSummary{}, rates, now)
		if e.Err != nil || e.Coverage.Covered.Amount != 3000 || e.Coverage.Remaining.Amount != 7000 || e.Coverage.CoveragePercent != 30 || e.Progress.Percent != 20 || e.Complete || !e.HasMonthlyNeeded || e.MonthlyNeeded.Amount != 3500 {
			t.Fatalf("foreign=%v: %+v", foreign, e)
		}
		if !reflect.DeepEqual(before, g.Allocations) {
			t.Fatal("evaluation changed persisted allocations")
		}
		if !foreign && (e.Coverage.CoveragePercent != CoveragePercent(g) || e.Pace != ClassifyPace(g, now)) {
			t.Fatal("same-currency parity changed")
		}
	}
}

func TestEvaluateCoverageUnknownAndOverflowAreNotCompletion(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		g    domain.Goal
		want error
	}{
		{"unknown", mkGoalWithAlloc("g", 10000, 9000, domain.GoalAllocation{Amount: money.New(1000, "EUR")}), currency.ErrUnknownRate},
		{"allocation-overflow", mkGoalWithAlloc("g", 10000, 9000, alloc("a", math.MaxInt64), alloc("b", 1)), money.ErrOverflow},
		{"coverage-overflow", mkGoalWithAlloc("g", 10000, 9000, alloc("a", math.MaxInt64)), money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.g.TargetDate = now.AddDate(0, 0, -1)
			e := EvaluateGoal(tc.g, TaskSummary{}, currency.Rates{Base: "USD"}, now)
			if !errors.Is(e.Err, tc.want) || e.Complete || e.State != StateCurrent || e.Pace != PaceNone || e.HasMonthlyNeeded || !e.Coverage.SavedKnown || e.Coverage.SavedPercent != 90 {
				t.Fatalf("unknown evaluation: %+v", e)
			}
			if CoverageMinor(tc.g) != 0 || CoveragePercent(tc.g) != 0 || Reached(tc.g, nil, now) || Classify(tc.g, nil, now) != StateCurrent || ClassifyPace(tc.g, now) != PaceNone {
				t.Fatal("rate-less compatibility helper inferred unknown coverage")
			}
		})
	}
}

func TestEvaluateCoverageAggregatesRoundingAndLegacyZero(t *testing.T) {
	g := mkGoalWithAlloc("g", 100, 0, domain.GoalAllocation{Amount: money.New(1, "EUR")}, domain.GoalAllocation{Amount: money.New(1, "EUR")})
	g.CurrentAmount = money.Money{}
	c, err := EvaluateCoverage(g, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.0 / 3}})
	if err != nil || c.SetAside.Amount != 1 || c.Covered.Amount != 1 || !c.SavedKnown {
		t.Fatalf("aggregate rounding: %+v, %v", c, err)
	}
	g.Allocations[0].Amount.Amount = -1
	if _, err := EvaluateCoverage(g, currency.Rates{}); err == nil {
		t.Fatal("negative reservation accepted")
	}
	g = mkGoalWithAlloc("g", math.MaxInt64, math.MaxInt64-1)
	c, err = EvaluateCoverage(g, currency.Rates{})
	if err != nil || c.SavedPercent != 99 || c.Remaining.Amount != 1 {
		t.Fatalf("large percent: %+v %v", c, err)
	}
}

func TestEvaluateGoalDeadlinePauseAndKinds(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	g := mkGoalWithAlloc("g", 10000, 0, alloc("a", 10000))
	g.TargetDate = now
	e := EvaluateGoal(g, TaskSummary{}, currency.Rates{}, now)
	if !e.Complete || e.State != StateCompleted || e.Pace != PaceComplete {
		t.Fatalf("fully reserved: %+v", e)
	}
	g.Allocations = nil
	g.PausedUntil = now.Add(time.Hour)
	if e := EvaluateGoal(g, TaskSummary{}, currency.Rates{}, now); e.State != StateCurrent || e.Pace != PacePaused {
		t.Fatalf("pause: %+v", e)
	}
	if e := EvaluateGoal(g, TaskSummary{}, currency.Rates{}, g.PausedUntil); e.State != StateMissed || e.Pace != PaceOverdue {
		t.Fatalf("exact pause end: %+v", e)
	}
	g.Kind = domain.GoalKindChecklist
	e = EvaluateGoal(g, TaskSummary{Done: 2, Total: 2}, currency.Rates{}, now)
	if !e.Complete || e.Progress.Percent != 100 || e.State != StateCompleted {
		t.Fatalf("checklist: %+v", e)
	}
}

func TestBuildTaskIndexPreviewAndNonmutation(t *testing.T) {
	tasks := []domain.Task{task("done1", "g", domain.StatusDone), task("open1", "g", domain.StatusOpen), task("done2", "g", domain.StatusDone), task("open2", "g", domain.StatusOpen), task("open3", "g", domain.StatusOpen), task("open4", "g", domain.StatusOpen), task("other", "other", domain.StatusDone)}
	before := append([]domain.Task(nil), tasks...)
	s := BuildTaskIndex(tasks)["g"]
	if s.Done != 2 || s.Total != 6 || len(s.Preview) != 3 || s.Preview[0].ID != "open1" || s.Preview[2].ID != "open3" {
		t.Fatalf("summary: %+v", s)
	}
	if !reflect.DeepEqual(before, tasks) {
		t.Fatal("task order mutated")
	}
	s = BuildTaskIndex(tasks[:3])["g"]
	if s.Preview[0].ID != "open1" || s.Preview[1].ID != "done1" || s.Preview[2].ID != "done2" {
		t.Fatalf("stable groups: %+v", s)
	}
}

func TestEvaluatedPlanUsesCoverageWithoutChangingGoal(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := mkGoalWithAlloc("g", 10000, 1000, domain.GoalAllocation{Amount: money.New(1000, "EUR")})
	g.TargetDate = now.AddDate(0, 7, 0)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	e := EvaluateGoal(g, TaskSummary{}, rates, now)
	if e.Err != nil || !e.HasMonthlyNeeded || e.MonthlyNeeded.Amount != 1000 {
		t.Fatalf("needed: %+v", e)
	}
	date, ok, err := e.Project(g, e.MonthlyNeeded, now)
	if err != nil || !ok || !date.Equal(now.AddDate(0, 6, 0)) {
		t.Fatalf("finish: %v %v %v", date, ok, err)
	}
	lo, hi, _, ok := e.SliderRange(g, rates)
	if !ok || lo.Amount != 250 || hi.Amount != 4000 {
		t.Fatalf("range: %v %v %v", lo, hi, ok)
	}
	g = mkGoalWithAlloc("tiny", 1, 0)
	e = EvaluateGoal(g, TaskSummary{}, rates, now)
	lo, hi, _, ok = e.SliderRange(g, rates)
	if !ok || lo.Amount != 1 || hi.Amount != 1 {
		t.Fatalf("one minor unit range: %v %v", lo, hi)
	}
	g = mkGoalWithAlloc("large", math.MaxInt64, 0)
	e = EvaluateGoal(g, TaskSummary{}, rates, now)
	g.MonthlyContribution = money.New(math.MaxInt64, "USD")
	lo, hi, _, ok = e.SliderRange(g, rates)
	if !ok || lo.Amount <= 0 || hi.Amount != math.MaxInt64 {
		t.Fatalf("overflowed range: %v %v", lo, hi)
	}
}
