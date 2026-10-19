// SPDX-License-Identifier: MIT

package goalfunded

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func allocationBudget(id, category string) domain.Budget {
	return domain.Budget{ID: id, CategoryID: category, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID}
}

func allocationGoal(id, category string) domain.Goal {
	return domain.Goal{ID: id, Name: id, CategoryID: category, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID}
}

func fundingMap(rows []BudgetFunding) map[string]BudgetFunding {
	out := make(map[string]BudgetFunding, len(rows))
	for _, row := range rows {
		out[row.BudgetID] = row
	}
	return out
}

func TestAllocateAvailable(t *testing.T) {
	categories := []domain.Category{
		{ID: "baby", Name: "Baby", Kind: domain.KindExpense},
		{ID: "wants", Name: "Wants", Kind: domain.KindExpense},
		{ID: "shopping", Name: "Shopping", Kind: domain.KindExpense, ParentID: "wants"},
		{ID: "splurges", Name: "Splurges", Kind: domain.KindExpense, ParentID: "wants"},
	}
	tests := []struct {
		name          string
		goal          domain.Goal
		budgets       []domain.Budget
		contributions []BudgetContribution
		want          map[string]BudgetFunding
	}{
		{
			name: "explicit budget claims the amount once and flags unrelated purpose",
			goal: func() domain.Goal {
				g := allocationGoal("baby-fund", "baby")
				g.Name = "Baby fund"
				g.BudgetIDs = []string{"splurges"}
				return g
			}(),
			budgets: []domain.Budget{allocationBudget("shopping", "shopping"), allocationBudget("splurges", "splurges")},
			want:    map[string]BudgetFunding{"splurges": {GoalID: "baby-fund", GoalName: "Baby fund", BudgetID: "splurges", AmountMinor: 42000, PurposeMismatch: true}},
		},
		{
			name:          "explicit split follows observed contribution share",
			goal:          func() domain.Goal { g := allocationGoal("g", ""); g.BudgetIDs = []string{"a", "b"}; return g }(),
			budgets:       []domain.Budget{allocationBudget("a", "x"), allocationBudget("b", "y")},
			contributions: []BudgetContribution{{GoalID: "g", BudgetID: "a", AmountMinor: 1}, {GoalID: "g", BudgetID: "b", AmountMinor: 2}},
			want: map[string]BudgetFunding{
				"a": {GoalID: "g", GoalName: "g", BudgetID: "a", AmountMinor: 14000},
				"b": {GoalID: "g", GoalName: "g", BudgetID: "b", AmountMinor: 28000},
			},
		},
		{
			name:          "category match selects one budget over contribution duplicates",
			goal:          allocationGoal("g", "shopping"),
			budgets:       []domain.Budget{allocationBudget("b-splurges", "splurges"), allocationBudget("a-shopping", "wants")},
			contributions: []BudgetContribution{{GoalID: "g", BudgetID: "a-shopping", AmountMinor: 1}, {GoalID: "g", BudgetID: "b-splurges", AmountMinor: 1}},
			want:          map[string]BudgetFunding{"a-shopping": {GoalID: "g", GoalName: "g", BudgetID: "a-shopping", AmountMinor: 42000}},
		},
		{
			name:          "unlinked goal splits by contribution with exact remainder",
			goal:          allocationGoal("g", ""),
			budgets:       []domain.Budget{allocationBudget("a", "x"), allocationBudget("b", "y")},
			contributions: []BudgetContribution{{GoalID: "g", BudgetID: "a", AmountMinor: 1}, {GoalID: "g", BudgetID: "b", AmountMinor: 2}},
			want: map[string]BudgetFunding{
				"a": {GoalID: "g", GoalName: "g", BudgetID: "a", AmountMinor: 14000},
				"b": {GoalID: "g", GoalName: "g", BudgetID: "b", AmountMinor: 28000},
			},
		},
		{
			name:    "no association or contribution remains unassigned",
			goal:    allocationGoal("g", "missing"),
			budgets: []domain.Budget{allocationBudget("a", "x"), allocationBudget("b", "y")},
			want:    map[string]BudgetFunding{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AllocateAvailable([]domain.Goal{tc.goal}, tc.budgets, categories, map[string]int64{tc.goal.ID: 42000}, tc.contributions)
			if !reflect.DeepEqual(fundingMap(got), tc.want) {
				t.Fatalf("allocation = %#v, want %#v", fundingMap(got), tc.want)
			}
			var assigned int64
			for _, row := range got {
				assigned += row.AmountMinor
			}
			if assigned > 42000 {
				t.Fatalf("allocated %d from only 42000 available", assigned)
			}
		})
	}
}

func TestAllocateAvailableRoundsAndOrdersDeterministically(t *testing.T) {
	goal := allocationGoal("g", "")
	goal.BudgetIDs = []string{"b", "a"}
	budgets := []domain.Budget{allocationBudget("b", "b-cat"), allocationBudget("a", "a-cat")}
	available := map[string]int64{"g": 2}
	want := []BudgetFunding{
		{GoalID: "g", GoalName: "g", BudgetID: "a", AmountMinor: 1},
		{GoalID: "g", GoalName: "g", BudgetID: "b", AmountMinor: 1},
	}
	for range 10 {
		got := AllocateAvailable([]domain.Goal{goal}, budgets, nil, available, nil)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("allocation = %#v, want deterministic %#v", got, want)
		}
	}
}

func TestAllocateAvailableNeverCrossesOwnerOrScope(t *testing.T) {
	goal := allocationGoal("g", "")
	goal.Scope, goal.OwnerID = domain.ScopeIndividual, "member-a"
	goal.BudgetIDs = []string{"shared", "member-b", "member-a"}
	budgets := []domain.Budget{
		allocationBudget("shared", "x"),
		{ID: "member-b", Scope: domain.ScopeIndividual, OwnerID: "member-b"},
		{ID: "member-a", Scope: domain.ScopeIndividual, OwnerID: "member-a"},
	}
	got := AllocateAvailable([]domain.Goal{goal}, budgets, nil, map[string]int64{"g": 100}, nil)
	if len(got) != 1 || got[0].BudgetID != "member-a" || got[0].AmountMinor != 100 {
		t.Fatalf("allocation crossed a scope/owner boundary: %+v", got)
	}
}
