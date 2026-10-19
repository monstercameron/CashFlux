// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestBudgetSuggestionUsesWeeklyAndPayAnchoredCadenceAndBudgetCurrency(t *testing.T) {
	app := br2sugTestApp(t)
	oldPrefs := uistate.LoadPrefs()
	t.Cleanup(func() { uistate.PersistPrefs(oldPrefs) })
	uistate.PersistPrefs(prefs.Prefs{WeekStart: prefs.WeekMonday, PayCycleAnchor: "2026-10-02"})
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 1.1}}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "biweekly-cat", Name: "Biweekly", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		category string
		period   domain.Period
		limit    int64
		spend    int64
	}{
		{name: "weekly", category: "transport-cat", period: domain.PeriodWeekly, limit: 30_000, spend: 10_000},
		{name: "biweekly pay cycle", category: "biweekly-cat", period: domain.PeriodBiweekly, limit: 60_000, spend: 20_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			budget := domain.Budget{ID: "suggest-" + tc.name, Name: "Transportation", CategoryID: tc.category, OwnerID: domain.GroupOwnerID,
				Scope: domain.ScopeShared, Period: tc.period, Limit: money.New(tc.limit, "EUR")}
			if err := app.PutBudget(budget); err != nil {
				t.Fatal(err)
			}
			start, _ := budgeting.PeriodRangeAnchored(tc.period, anchor, time.Monday, time.Date(2026, time.October, 2, 0, 0, 0, 0, time.UTC))
			for i := 0; i < suggestLimitMonths; i++ {
				start, _ = budgeting.PreviousPeriodRangeAnchored(tc.period, start, time.Monday, time.Date(2026, time.October, 2, 0, 0, 0, 0, time.UTC))
				if err := app.PutTransaction(domain.Transaction{ID: budget.ID + "-history-" + start.Format("2006-01-02"), AccountID: "br2sug-account",
					Desc: "Test expense", Date: start.Add(24 * time.Hour), CategoryID: tc.category, Amount: money.New(-tc.spend, "USD")}); err != nil {
					t.Fatal(err)
				}
			}
			got, displayed, ok := budgetRecommendationAt(app, budget, anchor)
			if !ok {
				t.Fatal("expected a history-backed suggestion")
			}
			if got.Direction != budgeting.SuggestionLower {
				t.Fatalf("recommendation direction = %q, want lower", got.Direction)
			}
			if displayed.Currency != "EUR" || displayed.Amount <= 0 || displayed.Amount >= tc.limit {
				t.Fatalf("displayed suggestion = %v, want a positive lower limit in EUR for %s cadence", displayed, tc.period)
			}
		})
	}
}
