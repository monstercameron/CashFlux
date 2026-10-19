// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestContextCategoryBudgetMedianIncludesDescendants(t *testing.T) {
	anchor := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	categories := []domain.Category{
		{ID: "transport", Kind: domain.KindExpense},
		{ID: "fuel", ParentID: "transport", Kind: domain.KindExpense},
	}
	txns := []domain.Transaction{
		{ID: "apr", CategoryID: "fuel", Amount: money.New(-10_000, "USD"), Date: time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)},
		{ID: "may", CategoryID: "fuel", Amount: money.New(-20_000, "USD"), Date: time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC)},
		{ID: "jun", CategoryID: "fuel", Amount: money.New(-30_000, "USD"), Date: time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)},
	}
	got, err := contextCategoryBudgetMedian("transport", categories, budgeting.NewNettedLedger(txns).Transactions(), anchor,
		currency.Rates{Base: "USD"}, time.Monday, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got != 20_000 {
		t.Fatalf("descendant median = %d, want 20000", got)
	}
}

func TestBudgetLimitRecommendationCopyNamesCadenceWindow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		period domain.Period
		want   string
	}{
		{"weekly", domain.PeriodWeekly, "Raise the limit to $20.00, based on the median of the last 3 weeks."},
		{"monthly", domain.PeriodMonthly, "Lower the limit to $20.00, based on the median of the last 3 months."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := budgetLimitRecommendationCopy(budgeting.SuggestionRaise, "$20.00", tc.period, 3, 0)
			if tc.name == "monthly" {
				got = budgetLimitRecommendationCopy(budgeting.SuggestionLower, "$20.00", tc.period, 3, 0)
			}
			if got != tc.want {
				t.Fatalf("copy = %q, want %q", got, tc.want)
			}
		})
	}
}
