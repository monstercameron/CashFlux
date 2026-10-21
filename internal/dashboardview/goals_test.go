// SPDX-License-Identifier: MIT

package dashboardview

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestDashboardGoalFactsUseCanonicalCoverageAndTaskCompletion(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	unknown := domain.Goal{ID: "unknown", Name: "Unknown", TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(2000, "JPY"), TargetDate: now.AddDate(0, 0, -1)}
	covered := domain.Goal{ID: "covered", Name: "Covered", TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(100, "USD"),
		Allocations: []domain.GoalAllocation{{AccountID: "foreign", Amount: money.New(450, "EUR")}}}
	checklist := domain.Goal{ID: "checklist", Name: "Checklist", Kind: domain.GoalKindChecklist}
	archived := covered
	archived.ID, archived.Archived = "archived", true
	input := []domain.Goal{unknown, covered, checklist, archived}
	before := append([]domain.Goal(nil), input...)
	before[1].Allocations = append([]domain.GoalAllocation(nil), covered.Allocations...)
	tasks := []domain.Task{{ID: "done", RelatedType: domain.RelatedGoal, RelatedID: checklist.ID, Status: domain.StatusDone}}
	facts := DeriveGoals(input, tasks, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, now)
	if len(facts.Rows) != 3 || facts.Counts.Current != 1 || facts.Counts.Completed != 2 || facts.Counts.Missed != 0 || facts.Unavailable != 1 {
		t.Fatalf("unknown FX or checklist facts produced a false lifecycle count: %+v", facts)
	}
	if e := facts.Rows[0].Evaluation; e.Err == nil || e.Complete || e.Pace != goals.PaceNone || e.State != goals.StateCurrent {
		t.Fatalf("unknown coverage became completed or overdue: %+v", e)
	}
	featured, ok := facts.Featured(true)
	if !ok || featured.Goal.ID != covered.ID || featured.Evaluation.Coverage.CoveragePercent != 100 {
		t.Fatalf("featured rank ignored converted reservations or input-order ties: %+v", featured)
	}
	if !reflect.DeepEqual(input, before) || input[0].CurrentAmount.Currency != "JPY" || covered.Allocations[0].Amount.Currency != "EUR" {
		t.Fatal("dashboard derivation rewrote source goal money")
	}
}

func TestDashboardGoalFactsExcludeOngoingFundsFromStateTally(t *testing.T) {
	goal := domain.Goal{ID: "fund", Name: "Fund", IsSinkingFund: true, TargetAmount: money.New(100, "USD"), CurrentAmount: money.New(100, "USD")}
	facts := DeriveGoals([]domain.Goal{goal}, nil, currency.Rates{Base: "USD"}, time.Now())
	if facts.Counts.Total() != 0 || len(facts.Rows) != 1 {
		t.Fatalf("ongoing fund became a finish-line lifecycle goal: %+v", facts)
	}
	if row, ok := facts.Featured(false); !ok || row.Goal.ID != goal.ID {
		t.Fatal("ongoing fund disappeared from the configured featured-goal tile")
	}
}
