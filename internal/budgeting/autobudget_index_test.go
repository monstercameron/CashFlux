// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestMonthlyBudgetsByCategory(t *testing.T) {
	tests := []struct {
		name    string
		budgets []domain.Budget
		wantIDs map[string]string
	}{
		{name: "empty input", wantIDs: map[string]string{}},
		{
			name: "omits non-monthly budgets",
			budgets: []domain.Budget{
				{ID: "weekly", CategoryID: "food", Period: domain.PeriodWeekly},
				{ID: "quarterly", CategoryID: "tax", Period: domain.PeriodQuarterly},
			},
			wantIDs: map[string]string{},
		},
		{
			name: "keeps monthly budgets by category",
			budgets: []domain.Budget{
				{ID: "food-monthly", CategoryID: "food", Period: domain.PeriodMonthly},
				{ID: "rent-monthly", CategoryID: "rent", Period: domain.PeriodMonthly},
			},
			wantIDs: map[string]string{"food": "food-monthly", "rent": "rent-monthly"},
		},
		{
			name: "last monthly budget wins for duplicate category",
			budgets: []domain.Budget{
				{ID: "earlier", CategoryID: "food", Period: domain.PeriodMonthly},
				{ID: "weekly", CategoryID: "food", Period: domain.PeriodWeekly},
				{ID: "later", CategoryID: "food", Period: domain.PeriodMonthly},
			},
			wantIDs: map[string]string{"food": "later"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MonthlyBudgetsByCategory(tt.budgets)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("got %d categories, want %d", len(got), len(tt.wantIDs))
			}
			for categoryID, wantID := range tt.wantIDs {
				if got[categoryID].ID != wantID {
					t.Errorf("category %q maps to %q, want %q", categoryID, got[categoryID].ID, wantID)
				}
			}
		})
	}
}
