// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAutoBudgetMatchesAnyCadenceAndTrackedCategoryWithoutTakingMemberBudget(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "member", CategoryID: "food", OwnerID: "alice", Period: domain.PeriodMonthly, Limit: money.New(1_000, "USD")},
		{ID: "multi", CategoryID: "rent", CategoryIDs: []string{"rent", "food"}, OwnerID: domain.GroupOwnerID, Period: domain.PeriodWeekly, Limit: money.New(5_000, "USD")},
		{ID: "exact", CategoryID: "food", OwnerID: domain.GroupOwnerID, Period: domain.PeriodYearly, Limit: money.New(60_000, "USD")},
	}
	got := AutoBudgetMatches(budgets)
	if got["food"].ID != "exact" {
		t.Fatalf("food matched %q, want exact household budget", got["food"].ID)
	}
	if got["rent"].ID != "multi" {
		t.Fatalf("rent matched %q, want multi-category weekly budget", got["rent"].ID)
	}
}

func TestFillAutoBudgetStateSeedsNewRowsAndPreservesEdits(t *testing.T) {
	suggestions := []BudgetSuggestion{{CategoryID: "old"}, {CategoryID: "new"}}
	percent := FillAutoBudgetPercentages(map[string]int{"old": 135}, suggestions)
	if percent["old"] != 135 || percent["new"] != 100 {
		t.Fatalf("percentages = %v, want old edit 135 and new default 100", percent)
	}
	selection := FillAutoBudgetSelection(map[string]bool{"old": false}, suggestions, map[string]domain.Budget{"new": {ID: "n"}})
	if selection["old"] || selection["new"] {
		t.Fatalf("selection = %v, want existing/edit state preserved and new compatible budget unselected", selection)
	}
}

func TestSelectedBudgetCategoriesRetainsIncomeCategoriesForSavings(t *testing.T) {
	categories := []domain.Category{
		{ID: "salary", Kind: domain.KindIncome},
		{ID: "food", Kind: domain.KindExpense},
	}
	selected := map[string]bool{"salary": true, "food": true}
	if got := SelectedBudgetCategories(categories, selected, false); len(got) != 1 || got[0] != "food" {
		t.Fatalf("spending categories = %v, want expense only", got)
	}
	if got := SelectedBudgetCategories(categories, selected, true); len(got) != 2 || got[0] != "salary" || got[1] != "food" {
		t.Fatalf("saving categories = %v, want selected income and expense categories", got)
	}
}

func TestBudgetCopyDefaultsRetainsCurrencyCadenceAndSavingDirection(t *testing.T) {
	b := domain.Budget{Limit: money.New(50_000, "JPY"), Period: domain.PeriodWeekly, Direction: domain.DirectionSave}
	got := CopyBudgetDefaults(b)
	if got.Limit != b.Limit || got.Period != b.Period || got.Direction != b.Direction {
		t.Fatalf("copy defaults = %+v, want currency, cadence, and save direction from source", got)
	}
}

func TestLimitForPeriodConvertsMonthlySuggestionToBudgetCadenceAndCurrency(t *testing.T) {
	got, err := LimitForPeriod(money.New(60_000, "USD"), domain.PeriodMonthly, domain.PeriodWeekly, "JPY", currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.0067}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Currency != "JPY" || got.Amount != 20_666 {
		t.Fatalf("weekly JPY equivalent = %v, want 20666 JPY units", got)
	}
}

func TestAutoBudgetImpactUsesHouseholdSpendingPlanAndNormalizesMatchedCadence(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "weekly", CategoryID: "food", OwnerID: domain.GroupOwnerID, Period: domain.PeriodWeekly, Limit: money.New(10_000, "USD")},
		{ID: "saving", CategoryID: "savings", OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Direction: domain.DirectionSave, Limit: money.New(20_000, "USD")},
		{ID: "alice", CategoryID: "car", OwnerID: "alice", Period: domain.PeriodMonthly, Limit: money.New(30_000, "USD")},
	}
	got, err := AutoBudgetImpactWithRates(budgets, 500_000, map[string]int64{"food": 60_000, "savings": 50_000}, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BudgetedBefore != 43_333 { // one weekly 10,000 limit normalized to monthly
		t.Fatalf("before = %d, want 43333", got.BudgetedBefore)
	}
	if got.BudgetedAfter != 59_999 {
		t.Fatalf("after = %d, want 59999 after weekly-period rounding; saving and member plans must not enter household spending", got.BudgetedAfter)
	}
}

func TestAutoBudgetImpactCombinesSelectedCategoriesSharingOneMultiCategoryBudget(t *testing.T) {
	budget := domain.Budget{ID: "household-food", CategoryID: "food", CategoryIDs: []string{"food", "dining"}, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(80_000, "USD")}
	got, err := AutoBudgetImpactWithRates([]domain.Budget{budget}, 0, map[string]int64{"food": 30_000, "dining": 20_000}, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BudgetedBefore != 80_000 || got.BudgetedAfter != 50_000 {
		t.Fatalf("multi-category impact = %d -> %d, want one combined replacement 80000 -> 50000", got.BudgetedBefore, got.BudgetedAfter)
	}
}

func TestGenerate5030WithRatesCountsSplitLinesAndConvertsCurrency(t *testing.T) {
	now := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "yen", CategoryID: "dining", Amount: money.New(-100_000, "JPY"), Date: time.Date(2026, time.May, 2, 0, 0, 0, 0, time.UTC)},
		{ID: "usd", CategoryID: "dining", Amount: money.New(-10_000, "USD"), Date: time.Date(2026, time.May, 3, 0, 0, 0, 0, time.UTC)},
		{ID: "split", CategoryID: "other", Amount: money.New(-30_000, "USD"), Date: time.Date(2026, time.May, 4, 0, 0, 0, 0, time.UTC), Splits: []domain.CategorySplit{{CategoryID: "dining", Amount: money.New(-20_000, "USD")}, {CategoryID: "other", Amount: money.New(-10_000, "USD")}}},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.0067}}
	got, err := trailingSpend("dining", txns, now, rates)
	if err != nil {
		t.Fatal(err)
	}
	if got != 97_000 {
		t.Fatalf("trailing dining spend = %d, want 97000 base minor units", got)
	}
	if _, err := Generate5030WithRates(100_000, []domain.Category{{ID: "dining", Name: "Dining", Kind: domain.KindExpense}}, txns, now, rates); err != nil {
		t.Fatal(err)
	}
}

func TestRecommendBudgetLimitWithContextUsesOwnerAndSelectedWeekStart(t *testing.T) {
	weekStart := time.Sunday
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "alice-food", CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "alice", Period: domain.PeriodWeekly, Limit: money.New(7_000, "USD")}
	txns := []domain.Transaction{
		{ID: "alice", CategoryID: "food", MemberID: "alice", Amount: money.New(-7_000, "USD"), Date: time.Date(2026, time.July, 5, 12, 0, 0, 0, time.UTC)},
		{ID: "household", CategoryID: "food", Amount: money.New(-90_000, "USD"), Date: time.Date(2026, time.July, 5, 12, 0, 0, 0, time.UTC)},
	}
	got, err := RecommendBudgetLimitWithContext(budget, nil, txns, now, 1, money.New(7_000, "USD"), currency.Rates{Base: "USD"}, weekStart, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Direction != SuggestionKeep || got.Median != 30_333 {
		t.Fatalf("recommendation = %+v, want only Alice's Sunday-week spending normalized to 30333 monthly", got)
	}
}
