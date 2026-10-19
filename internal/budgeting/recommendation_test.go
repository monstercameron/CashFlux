// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func recommendMonthlyForTest(covers map[string]bool, txns []domain.Transaction, now time.Time, periods int, limit, projection int64, rates currency.Rates) (LimitRecommendation, error) {
	budget := domain.Budget{Period: domain.PeriodMonthly, Limit: money.New(limit, rates.Base)}
	return RecommendBudgetLimitWithContext(budget, covers, NewNettedLedger(txns).Transactions(), now, periods,
		money.New(projection, rates.Base), rates, time.Monday, time.Time{})
}

func TestRecommendLimitNamesDirectionAndExcludesOutlier(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	amounts := []int64{20000, 21000, 19000, 12642, 20000, 22000}
	txns := make([]domain.Transaction, 0, len(amounts))
	for i, amount := range amounts {
		month := time.Date(2026, time.Month(6-i), 10, 0, 0, 0, 0, time.UTC)
		txns = append(txns, domain.Transaction{ID: month.Format("2006-01"), CategoryID: "transport", Amount: money.New(-amount, "USD"), Date: month})
	}
	got, err := recommendMonthlyForTest(map[string]bool{"transport": true}, txns, now, 6, 15000, 0, currency.Rates{Base: "USD"})
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

func TestRecommendLimitProjectionNeverDeterminesSuggestedAmount(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "a", CategoryID: "dining", Amount: money.New(-12000, "USD"), Date: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", CategoryID: "dining", Amount: money.New(-12642, "USD"), Date: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "c", CategoryID: "dining", Amount: money.New(-12600, "USD"), Date: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)},
	}
	got, err := recommendMonthlyForTest(map[string]bool{"dining": true}, txns, now, 3, 30000, 188301, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Direction != SuggestionLower || got.Suggested != 12600 || got.Projection != 188301 {
		t.Errorf("projection-derived recommendation = %+v, want historical 12600 and recorded projection 188301", got)
	}
}

func TestRecommendLimitSuppressesRaiseWithoutRepeatedOverspend(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		lastThree  []int64
		projection int64
		want       SuggestionDirection
	}{
		{"on pace with one overspend", []int64{12000, 8000, 8000}, 9000, SuggestionKeep},
		{"on pace with repeated overspend", []int64{12000, 11000, 8000}, 9000, SuggestionRaise},
		{"currently over pace", []int64{12000, 8000, 8000}, 11000, SuggestionRaise},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amounts := []int64{20000, 20000, tc.lastThree[0], tc.lastThree[1], tc.lastThree[2]}
			txns := make([]domain.Transaction, 0, len(amounts))
			for i, amount := range amounts {
				month := time.Date(2026, time.Month(i+2), 10, 0, 0, 0, 0, time.UTC)
				txns = append(txns, domain.Transaction{ID: month.Format("2006-01"), CategoryID: "shopping", Amount: money.New(-amount, "USD"), Date: month})
			}
			got, err := recommendMonthlyForTest(map[string]bool{"shopping": true}, txns, now, 5, 10000, tc.projection, currency.Rates{Base: "USD"})
			if err != nil || got.Direction != tc.want || (got.Direction == SuggestionRaise && got.Suggested != got.Median) {
				t.Fatalf("recommendation=%+v err=%v; want %s with evidence-derived amount", got, err, tc.want)
			}
		})
	}
}

func TestRecommendLimitCanSuggestALowerLimitWhenProjectionFits(t *testing.T) {
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "a", CategoryID: "food", Amount: money.New(-20000, "USD"), Date: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", CategoryID: "food", Amount: money.New(-21000, "USD"), Date: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)},
	}
	got, err := recommendMonthlyForTest(map[string]bool{"food": true}, txns, now, 2, 30000, 15000, currency.Rates{Base: "USD"})
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
	got, err := RecommendBudgetLimitWithContext(budget, map[string]bool{"travel": true}, txns, now, 3, money.New(40000, "USD"), currency.Rates{Base: "USD"}, time.Monday, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentLimit != 50000 || got.Direction != SuggestionKeep || got.Suggested != 50000 {
		t.Errorf("yearly budget recommendation = current %d, %s to %d; want monthly equivalent 50000 and keep", got.CurrentLimit, got.Direction, got.Suggested)
	}
}

func TestRecommendBudgetLimitUsesComparableYearWindows(t *testing.T) {
	now := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "2023", CategoryID: "splurge", Amount: money.New(-900000, "USD"), Date: time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "2024", CategoryID: "splurge", Amount: money.New(-1000000, "USD"), Date: time.Date(2024, time.June, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "2025", CategoryID: "splurge", Amount: money.New(-800000, "USD"), Date: time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC)},
		// A large current-year transaction must not leak into the closed-year evidence.
		{ID: "2026", CategoryID: "splurge", Amount: money.New(-500000, "USD"), Date: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
	}
	budget := domain.Budget{ID: "splurge", Period: domain.PeriodYearly, Limit: money.New(888000, "USD")}
	got, err := RecommendBudgetLimitWithContext(budget, map[string]bool{"splurge": true}, txns, now, 3, money.New(40000, "USD"), currency.Rates{Base: "USD"}, time.Monday, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.MonthsUsed) != 3 || got.MonthsUsed[0].Month != "2023-01-01" || got.MonthsUsed[2].Month != "2025-01-01" || got.Median != 75000 || got.Direction != SuggestionRaise || got.Suggested != 75000 {
		t.Fatalf("yearly evidence=%+v; want three comparable annual windows and a monthly-equivalent median of 75000", got)
	}
}

func TestRecommendBudgetLimitUsesComparableQuarterWindows(t *testing.T) {
	now := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "q1", CategoryID: "home", Amount: money.New(-330000, "USD"), Date: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "q2", CategoryID: "home", Amount: money.New(-300000, "USD"), Date: time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "q3", CategoryID: "home", Amount: money.New(-360000, "USD"), Date: time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)},
		{ID: "current", CategoryID: "home", Amount: money.New(-900000, "USD"), Date: time.Date(2026, time.October, 2, 0, 0, 0, 0, time.UTC)},
	}
	budget := domain.Budget{ID: "home", Period: domain.PeriodQuarterly, Limit: money.New(300000, "USD")}
	got, err := RecommendBudgetLimitWithContext(budget, map[string]bool{"home": true}, txns, now, 3, money.New(80000, "USD"), currency.Rates{Base: "USD"}, time.Monday, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.MonthsUsed) != 3 || got.MonthsUsed[0].Month != "2026-01-01" || got.MonthsUsed[2].Month != "2026-07-01" || got.Median != 110000 || got.Direction != SuggestionRaise || got.Suggested != 110000 {
		t.Fatalf("quarterly evidence=%+v; want three comparable quarters and a monthly-equivalent median of 110000", got)
	}
}

func TestRecommendLimitWithoutHistoryKeepsCurrentLimit(t *testing.T) {
	got, err := recommendMonthlyForTest(map[string]bool{"new": true}, nil, time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), 6, 20000, 0, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.HasEvidence || got.Direction != SuggestionKeep || got.Suggested != 20000 {
		t.Errorf("no-history recommendation = %+v, want keep current 20000 with no evidence", got)
	}
}
