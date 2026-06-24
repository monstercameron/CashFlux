// SPDX-License-Identifier: MIT

package allocate

import "testing"

func TestPlanActionsGoalPrefixStripping(t *testing.T) {
	plans := []Plan{
		{Candidate: Candidate{ID: "goal:g1", Name: "Goal · Retirement"}, Amount: 500},
	}
	actions := PlanActions(plans, nil)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Kind != GoalContribution {
		t.Errorf("kind = %d, want GoalContribution", a.Kind)
	}
	if a.DestinationID != "g1" {
		t.Errorf("DestinationID = %q, want %q", a.DestinationID, "g1")
	}
	if a.Amount != 500 {
		t.Errorf("Amount = %d, want 500", a.Amount)
	}
}

func TestPlanActionsLiabilityClassification(t *testing.T) {
	isLiability := func(id string) bool { return id == "credit-card" }
	plans := []Plan{
		{Candidate: Candidate{ID: "savings", Name: "Savings"}, Amount: 300},
		{Candidate: Candidate{ID: "credit-card", Name: "Credit Card"}, Amount: 200},
	}
	actions := PlanActions(plans, isLiability)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].Kind != AccountEarmark {
		t.Errorf("savings kind = %d, want AccountEarmark", actions[0].Kind)
	}
	if actions[1].Kind != DebtPaydownEarmark {
		t.Errorf("credit-card kind = %d, want DebtPaydownEarmark", actions[1].Kind)
	}
}

func TestPlanActionsDropsZeroAmount(t *testing.T) {
	plans := []Plan{
		{Candidate: Candidate{ID: "a", Name: "A"}, Amount: 0},
		{Candidate: Candidate{ID: "b", Name: "B"}, Amount: 100},
		{Candidate: Candidate{ID: "c", Name: "C"}, Amount: -5},
	}
	actions := PlanActions(plans, nil)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action (only B), got %d", len(actions))
	}
	if actions[0].DestinationID != "b" {
		t.Errorf("expected B, got %q", actions[0].DestinationID)
	}
}

func TestPlanActionsEmptyInput(t *testing.T) {
	actions := PlanActions(nil, nil)
	if len(actions) != 0 {
		t.Errorf("expected empty, got %d", len(actions))
	}
}

func TestPlanActionsAmountInvariant(t *testing.T) {
	// sum(Action.Amount) == sum(plan.Amount for Amount > 0)
	plans := []Plan{
		{Candidate: Candidate{ID: "goal:g1"}, Amount: 400},
		{Candidate: Candidate{ID: "acc1"}, Amount: 300},
		{Candidate: Candidate{ID: "acc2"}, Amount: 0},   // dropped
		{Candidate: Candidate{ID: "acc3"}, Amount: -10}, // dropped
		{Candidate: Candidate{ID: "acc4"}, Amount: 100},
	}
	actions := PlanActions(plans, nil)
	var sumActions int64
	for _, a := range actions {
		sumActions += a.Amount
	}
	var sumPlans int64
	for _, p := range plans {
		if p.Amount > 0 {
			sumPlans += p.Amount
		}
	}
	if sumActions != sumPlans {
		t.Errorf("invariant broken: sumActions=%d sumPlans=%d", sumActions, sumPlans)
	}
}

func TestPlanActionsNilClassifierTreatsAssetEarmark(t *testing.T) {
	// When isLiability is nil, all non-goal candidates become AccountEarmark.
	plans := []Plan{
		{Candidate: Candidate{ID: "debt-acc"}, Amount: 150},
	}
	actions := PlanActions(plans, nil)
	if actions[0].Kind != AccountEarmark {
		t.Errorf("nil classifier: kind = %d, want AccountEarmark", actions[0].Kind)
	}
}
