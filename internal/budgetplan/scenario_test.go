// SPDX-License-Identifier: MIT

package budgetplan

import (
	"reflect"
	"testing"
)

func TestEvaluateWaterfall(t *testing.T) {
	plans := []BudgetPlan{
		{BudgetID: "a", Name: "Rent", PlanMinor: 40000},
		{BudgetID: "b", Name: "Food", PlanMinor: 40000},
		{BudgetID: "c", Name: "Fun", PlanMinor: 40000},
	}
	tests := []struct {
		name            string
		in              ScenarioInput
		wantAdjIncome   int64
		wantTotalPlan   int64
		wantShortfall   int64
		wantUnderfunded []string
	}{
		{
			name:            "income below plan cuts the last budget",
			in:              ScenarioInput{IncomeMinor: 100000, Plans: plans},
			wantAdjIncome:   100000,
			wantTotalPlan:   120000,
			wantShortfall:   20000,
			wantUnderfunded: []string{"c"},
		},
		{
			name:            "negative income delta deepens the shortfall",
			in:              ScenarioInput{IncomeMinor: 120000, IncomeDeltaMinor: -30000, Plans: plans},
			wantAdjIncome:   90000,
			wantTotalPlan:   120000,
			wantShortfall:   30000,
			wantUnderfunded: []string{"c"},
		},
		{
			name:            "shortfall spanning two budgets",
			in:              ScenarioInput{IncomeMinor: 120000, IncomeDeltaMinor: -50000, Plans: plans},
			wantAdjIncome:   70000,
			wantTotalPlan:   120000,
			wantShortfall:   50000,
			wantUnderfunded: []string{"b", "c"}, // b partially funded, c fully unfunded
		},
		{
			name:            "positive delta funds everything",
			in:              ScenarioInput{IncomeMinor: 100000, IncomeDeltaMinor: 50000, Plans: plans},
			wantAdjIncome:   150000,
			wantTotalPlan:   120000,
			wantShortfall:   0,
			wantUnderfunded: nil,
		},
		{
			name:            "category delta bumps one budget's plan",
			in:              ScenarioInput{IncomeMinor: 80000, Plans: plans[:2], TargetBudgetID: "b", CategoryDeltaMinor: 20000},
			wantAdjIncome:   80000,
			wantTotalPlan:   100000, // 40000 + (40000+20000)
			wantShortfall:   20000,
			wantUnderfunded: []string{"b"},
		},
		{
			name:            "negative income floors at zero",
			in:              ScenarioInput{IncomeMinor: 0, IncomeDeltaMinor: -500, Plans: plans[:1]},
			wantAdjIncome:   0,
			wantTotalPlan:   40000,
			wantShortfall:   40000,
			wantUnderfunded: []string{"a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.in)
			if got.AdjustedIncomeMinor != tt.wantAdjIncome {
				t.Errorf("AdjustedIncome = %d, want %d", got.AdjustedIncomeMinor, tt.wantAdjIncome)
			}
			if got.TotalPlanMinor != tt.wantTotalPlan {
				t.Errorf("TotalPlan = %d, want %d", got.TotalPlanMinor, tt.wantTotalPlan)
			}
			if got.ShortfallMinor != tt.wantShortfall {
				t.Errorf("Shortfall = %d, want %d", got.ShortfallMinor, tt.wantShortfall)
			}
			if !reflect.DeepEqual(got.Underfunded, tt.wantUnderfunded) {
				t.Errorf("Underfunded = %v, want %v", got.Underfunded, tt.wantUnderfunded)
			}
		})
	}
}

func TestEvaluateFundedBreakdown(t *testing.T) {
	in := ScenarioInput{
		IncomeMinor: 50000,
		Plans: []BudgetPlan{
			{BudgetID: "a", PlanMinor: 30000},
			{BudgetID: "b", PlanMinor: 30000},
		},
	}
	got := Evaluate(in)
	if len(got.Funded) != 2 {
		t.Fatalf("expected 2 funded rows, got %d", len(got.Funded))
	}
	// a fully funded, b gets the remaining 20000 and is short 10000.
	if f := got.Funded[0]; f.FundedMinor != 30000 || f.Underfunded() {
		t.Errorf("row a = %+v", f)
	}
	if f := got.Funded[1]; f.FundedMinor != 20000 || f.ShortfallMinor != 10000 || !f.Underfunded() {
		t.Errorf("row b = %+v", f)
	}
	if !got.IsUnderfunded("b") || got.IsUnderfunded("a") {
		t.Errorf("IsUnderfunded wrong: %v", got.Underfunded)
	}
}
