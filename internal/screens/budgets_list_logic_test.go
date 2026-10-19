// SPDX-License-Identifier: MIT

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetListSearchVisible(t *testing.T) {
	for _, tt := range []struct {
		count int
		want  bool
	}{{0, false}, {5, false}, {6, true}, {7, true}} {
		if got := budgetListSearchVisible(tt.count); got != tt.want {
			t.Errorf("budgetListSearchVisible(%d) = %t, want %t", tt.count, got, tt.want)
		}
	}
}

func TestFilterBudgetStatuses(t *testing.T) {
	makeStatus := func(id, name string, spent int64) budgeting.Status {
		b := domain.Budget{ID: id, Name: name, CategoryID: "food", TrackedTags: []string{"weekly"}}
		limit := money.New(100, "USD")
		status := budgeting.Status{Budget: b, Limit: limit, Spent: money.New(spent, "USD"), Remaining: money.New(100-spent, "USD")}
		if spent > 100 {
			status.State = budgeting.StateOver
		}
		return status
	}
	statuses := []budgeting.Status{makeStatus("one", "Groceries", 120), makeStatus("two", "Dining", 30)}
	categories := map[string]string{"food": "Food"}
	tests := []struct {
		name, query, attention string
		want                   []string
	}{
		{name: "empty query", want: []string{"one", "two"}},
		{name: "trimmed case insensitive name", query: "  GROC ", want: []string{"one"}},
		{name: "category", query: "food", want: []string{"one", "two"}},
		{name: "tag", query: "weekly", want: []string{"one", "two"}},
		{name: "attention", attention: "needs-attention", want: []string{"one"}},
		{name: "unknown attention key", attention: "unknown", want: []string{}},
		{name: "search and attention intersect", query: "Dining", attention: "needs-attention", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterBudgetStatuses(statuses, categories, tt.query, tt.attention)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d statuses, want %d", len(got), len(tt.want))
			}
			for i, id := range tt.want {
				if got[i].Budget.ID != id {
					t.Errorf("status %d id = %q, want %q", i, got[i].Budget.ID, id)
				}
			}
		})
	}
}
