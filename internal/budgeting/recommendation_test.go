// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRecommendLimitNamesDirectionAndExcludesOutlier(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	amounts := []int64{20000, 21000, 19000, 12642, 20000, 22000}
	txns := make([]domain.Transaction, 0, len(amounts))
	for i, amount := range amounts {
		month := time.Date(2026, time.Month(6-i), 10, 0, 0, 0, 0, time.UTC)
		txns = append(txns, domain.Transaction{ID: month.Format("2006-01"), CategoryID: "transport", Amount: money.New(-amount, "USD"), Date: month})
	}
	got, err := RecommendLimit(map[string]bool{"transport": true}, txns, now, 6, 15000, 0, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Direction != SuggestionRaise || got.Suggested != 20000 {
		t.Errorf("recommendation = %s to %d, want raise to 20000", got.Direction, got.Suggested)
	}
	if len(got.Outliers) != 1 || got.Outliers[0].Amount != 12642 || len(got.MonthsUsed) != 5 {
		t.Errorf("evidence used=%+v excluded=%+v, want five used and the 12642 outlier excluded", got.MonthsUsed, got.Outliers)
	}
}

func TestRecommendLimitNeverCutsBelowCurrentProjection(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "a", CategoryID: "dining", Amount: money.New(-12000, "USD"), Date: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", CategoryID: "dining", Amount: money.New(-12642, "USD"), Date: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "c", CategoryID: "dining", Amount: money.New(-12600, "USD"), Date: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)},
	}
	got, err := RecommendLimit(map[string]bool{"dining": true}, txns, now, 3, 30000, 188301, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Direction != SuggestionRaise || got.Suggested != 188301 {
		t.Errorf("projected recommendation = %s to %d, want raise to at least projected 188301", got.Direction, got.Suggested)
	}
}

func TestRecommendLimitCanSuggestALowerLimitWhenProjectionFits(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "a", CategoryID: "food", Amount: money.New(-20000, "USD"), Date: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", CategoryID: "food", Amount: money.New(-21000, "USD"), Date: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)},
	}
	got, err := RecommendLimit(map[string]bool{"food": true}, txns, now, 2, 30000, 15000, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Direction != SuggestionLower || got.Suggested != 20500 {
		t.Errorf("recommendation = %s to %d, want lower to median 20500", got.Direction, got.Suggested)
	}
}

func TestRecommendBudgetLimitComparesYearlyBaseAsMonthlyEquivalent(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "a", CategoryID: "travel", Amount: money.New(-50000, "USD"), Date: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", CategoryID: "travel", Amount: money.New(-52000, "USD"), Date: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "c", CategoryID: "travel", Amount: money.New(-48000, "USD"), Date: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)},
	}
	budget := domain.Budget{ID: "travel", Period: domain.PeriodYearly, Limit: money.New(600000, "USD")}
	got, err := RecommendBudgetLimit(budget, map[string]bool{"travel": true}, txns, now, 3, money.New(40000, "USD"), currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentLimit != 50000 || got.Direction != SuggestionKeep || got.Suggested != 50000 {
		t.Errorf("yearly budget recommendation = current %d, %s to %d; want monthly equivalent 50000 and keep", got.CurrentLimit, got.Direction, got.Suggested)
	}
}

func TestRecommendLimitWithoutHistoryKeepsCurrentLimit(t *testing.T) {
	got, err := RecommendLimit(map[string]bool{"new": true}, nil, time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), 6, 20000, 0, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.HasEvidence || got.Direction != SuggestionKeep || got.Suggested != 20000 {
		t.Errorf("no-history recommendation = %+v, want keep current 20000 with no evidence", got)
	}
}
