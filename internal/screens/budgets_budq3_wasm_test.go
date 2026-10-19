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
	gotTxns, gotCovers := suggestTransactionsForScope(covers, []string{"vacation"}, txns)
	if len(gotTxns) != len(txns)-1 {
		t.Fatalf("adapted transaction count = %d, want %d after report exclusion", len(gotTxns), len(txns)-1)
	}
	if !gotCovers["groceries"] || !gotCovers["__budget_tag__tagged-other-category"] {
		t.Fatalf("scope omitted category or tag coverage: %v", gotCovers)
	}
	if gotCovers["travel"] {
		t.Fatal("tag adaptation broadened scope to every transaction in the tagged transaction's category")
	}
	if gotTxns[1].CategoryID != "__budget_tag__tagged-other-category" || gotTxns[2].CategoryID != "travel" {
		t.Fatalf("tag adaptation changed the wrong transaction categories: %+v", gotTxns)
	}
}

func TestBUDQ3TagSuggestionCountsFullTaggedChargeButNotExcludedCharge(t *testing.T) {
	anchor := time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "category-spend", Date: time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC), CategoryID: "groceries", Amount: money.New(-5000, "USD")},
		{ID: "tag-spend", Date: time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC), CategoryID: "travel", Tags: []string{"vacation"}, Amount: money.New(-30000, "USD")},
		{ID: "excluded-tag", Date: time.Date(2026, time.August, 6, 0, 0, 0, 0, time.UTC), CategoryID: "travel", Tags: []string{"vacation"}, Amount: money.New(-90000, "USD"), ExcludeFromReports: true},
	}
	adapted, covers := suggestTransactionsForScope(map[string]bool{"groceries": true}, []string{"vacation"}, txns)
	got, err := budgeting.SuggestLimitIn(covers, adapted, anchor, suggestLimitMonths, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("SuggestLimitIn: %v", err)
	}
	if got != 17500 {
		t.Fatalf("suggested monthly spend = %d cents, want 17500 (the $350 August scope averaged over August and September, excluding $900)", got)
	}
}
