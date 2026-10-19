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

func TestBUDQ3AnnualPlanTotalUsesBudgetingCadence(t *testing.T) {
	b := domain.Budget{
		ID: "semi-monthly", Name: "Groceries", Period: domain.PeriodSemimonthly,
		Limit: money.New(20000, "USD"),
	}
	if got := annualPlanTotal([]domain.Budget{b}, "USD"); got != 480000 {
		t.Fatalf("annualPlanTotal(semimonthly $200) = %d cents, want 480000", got)
	}
}

func TestBUDQ3AnnualMonthLabelUsesCatalog(t *testing.T) {
	if got := annualMonthLabel(0, 2026); got != "January 2026" {
		t.Fatalf("annualMonthLabel(January) = %q", got)
	}
	if got := annualMonthLabel(11, 2026); got != "December 2026" {
		t.Fatalf("annualMonthLabel(December) = %q", got)
	}
}

func TestBUDQ3SuggestionScopeIncludesTrackedTagsAndCategories(t *testing.T) {
	covers := map[string]bool{"groceries": true}
	txns := []domain.Transaction{
		{ID: "category", CategoryID: "groceries"},
		{ID: "tagged-other-category", CategoryID: "travel", Tags: []string{" Vacation "}},
		{ID: "unmatched", CategoryID: "travel"},
		{ID: "excluded-tag", CategoryID: "travel", Tags: []string{"vacation"}, ExcludeFromReports: true},
	}
	b := domain.Budget{CategoryID: "groceries", TrackedTags: []string{"vacation"}, Limit: money.New(50000, "USD")}
	for i, txn := range txns {
		txn.Amount = money.New(-100, "USD")
		amount, ok, err := budgeting.ContributionChecked(b, txn, func(id string) bool { return covers[id] })
		want := i < 2
		if err != nil || ok != want || ok && amount.Amount != 100 {
			t.Fatalf("%s contribution=%+v ok=%v err=%v", txn.ID, amount, ok, err)
		}
	}
	if txns[1].CategoryID != "travel" || txns[2].CategoryID != "travel" {
		t.Fatal("scope evaluation mutated categories")
	}
}

func TestBUDQ3TagSuggestionCountsFullTaggedChargeButNotExcludedCharge(t *testing.T) {
	anchor := time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "category-spend", Date: time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC), CategoryID: "groceries", Amount: money.New(-5000, "USD")},
		{ID: "tag-spend", Date: time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC), CategoryID: "travel", Tags: []string{"vacation"}, Amount: money.New(-30000, "USD")},
		{ID: "excluded-tag", Date: time.Date(2026, time.August, 6, 0, 0, 0, 0, time.UTC), CategoryID: "travel", Tags: []string{"vacation"}, Amount: money.New(-90000, "USD"), ExcludeFromReports: true},
	}
	b := domain.Budget{CategoryID: "groceries", TrackedTags: []string{"vacation"}, Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")}
	ledger := budgeting.NewNettedLedger(txns)
	got, err := budgeting.RecommendBudgetLimitWithContext(b, map[string]bool{"groceries": true}, ledger, anchor, suggestLimitMonths, money.Zero("USD"), currency.Rates{Base: "USD"}, time.Sunday, time.Time{})
	if err != nil {
		t.Fatalf("SuggestLimitIn: %v", err)
	}
	if got.Suggested != 17500 {
		t.Fatalf("suggested monthly spend = %d cents, want 17500 (the $350 August scope with September zero, excluding $900)", got.Suggested)
	}
}
