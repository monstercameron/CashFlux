// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestGoalContributionHistoryRendersUnavailableForMixedCurrency(t *testing.T) {
	g := domain.Goal{
		ID: "goal-history-fx", TargetAmount: money.New(10000, "USD"),
		Contributions: []domain.GoalContribution{{Amount: money.New(100, "EUR"), At: time.Now()}},
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node { return goalContribHistoryNode(g, money.Zero("USD"), false, nil, time.Now()) }))
	f.Stabilize()
	if byTestID(f, "p", "goal-hist-unavailable-"+g.ID) == nil || byTestID(f, "div", "goal-hist-"+g.ID) != nil {
		t.Fatalf("mixed-currency history was drawn as target currency: %s", f.Text())
	}
}

func TestGoalContributionHistoryRetainsValidRows(t *testing.T) {
	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	g := domain.Goal{
		ID: "goal-history-valid", TargetAmount: money.New(10000, "USD"),
		Contributions: []domain.GoalContribution{{Amount: money.New(5000, "USD"), At: now}},
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node { return goalContribHistoryNode(g, money.Zero("USD"), false, nil, now) }))
	f.Stabilize()
	if byTestID(f, "div", "goal-hist-"+g.ID) == nil || byTestID(f, "p", "goal-hist-unavailable-"+g.ID) != nil || !strings.Contains(f.Text(), "$50.00") {
		t.Fatalf("valid contribution history disappeared: %s", f.Text())
	}
}

func TestGoalContributionHistoryShowsUnavailableForAssignmentError(t *testing.T) {
	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	g := domain.Goal{
		ID: "goal-history-plan-error", TargetAmount: money.New(10000, "USD"),
		Contributions: []domain.GoalContribution{{Amount: money.New(5000, "USD"), At: now}},
	}
	e := goalsvc.Evaluation{}
	assignment, has, err := e.Assignment(g, goalRates(nil))
	if err != nil || has {
		t.Fatalf("empty assignment = (%+v, %t, %v), want no plan", assignment, has, err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		return goalContribHistoryNode(g, assignment, has, errors.New("assignment failed"), now)
	}))
	f.Stabilize()
	if byTestID(f, "p", "goal-hist-unavailable-"+g.ID) == nil || byTestID(f, "div", "goal-hist-"+g.ID) != nil {
		t.Fatalf("failed assignment was rendered as a plan/no-plan history: %s", f.Text())
	}
}

func TestGoalCardHistoryUsesCanonicalFXAdjustedDeadlinePlan(t *testing.T) {
	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.2}}
	g := domain.Goal{
		ID: "goal-history-fx-plan", TargetAmount: money.New(10000, "USD"),
		TargetDate:    now.AddDate(0, 2, 0),
		Allocations:   []domain.GoalAllocation{{AccountID: "foreign-account", Amount: money.New(1000, "EUR")}},
		Contributions: []domain.GoalContribution{{Amount: money.New(100, "USD"), At: now}},
	}
	e := goalsvc.EvaluateGoal(g, goalsvc.TaskSummary{}, rates, now)
	if e.Err != nil || !e.HasMonthlyNeeded || e.MonthlyNeeded.Amount != 4400 {
		t.Fatalf("canonical deadline plan = %+v, want $44.00/mo: %v", e.MonthlyNeeded, e.Err)
	}
	details := buildGoalCardDetails(g, e, nil, rates, now)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node { return details.History }))
	f.Stabilize()
	if byTestID(f, "p", "goal-hist-unavailable-"+g.ID) != nil || byTestID(f, "div", "goal-hist-"+g.ID) == nil || !strings.Contains(f.Text(), "$44.00") {
		t.Fatalf("history omitted FX-adjusted deadline plan: %s", f.Text())
	}
}
