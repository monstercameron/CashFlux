// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAutoBudgetSaveUpdatesWeeklyMatchWithoutCreatingDuplicate(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	original := domain.Budget{ID: "weekly-food", Name: "Food", CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodWeekly, Limit: money.New(10_000, "USD")}
	if err := app.PutBudget(original); err != nil {
		t.Fatal(err)
	}
	rows := []budgeting.BudgetSuggestion{{CategoryID: "food", CategoryName: "Food", MonthlyMinor: 60_000}}
	matched := budgeting.AutoBudgetMatches(app.Budgets())
	tuning := budgeting.PrepareAutoBudgetTuning(rows, nil, map[string]bool{"food": true})
	saveAutoBudget(app, tuning, matched, "USD", ratesUSD(), time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), nil, nil, nil, func() {})
	budgets := app.Budgets()
	if len(budgets) != 1 || budgets[0].ID != original.ID {
		t.Fatalf("budgets after update = %+v, want only existing weekly budget", budgets)
	}
	if budgets[0].Period != domain.PeriodWeekly || budgets[0].Limit.Amount != 13_846 {
		t.Fatalf("updated budget = %+v, want same weekly cadence with period-equivalent 13846 limit", budgets[0])
	}
}

func TestAutoBudgetSaveIsAtomicWhenOneSelectedBudgetIsInvalid(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []domain.Category{
		{ID: "valid", Name: "Valid", Kind: domain.KindExpense},
		{ID: "invalid", Name: "Invalid", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	rows := []budgeting.BudgetSuggestion{
		{CategoryID: "valid", CategoryName: "Valid", MonthlyMinor: 5_000},
		{CategoryID: "invalid", CategoryName: "", MonthlyMinor: 5_000},
	}
	closed := false
	tuning := budgeting.PrepareAutoBudgetTuning(rows, nil, map[string]bool{"valid": true, "invalid": true})
	saveAutoBudget(app, tuning, nil, "USD", ratesUSD(), time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), nil, nil, nil, func() { closed = true })
	if got := app.Budgets(); len(got) != 0 {
		t.Fatalf("partial AutoBudget write left budgets %+v; want no writes when one row fails", got)
	}
	if closed {
		t.Fatal("AutoBudget closed after a rejected atomic batch")
	}
}

func TestBudgetTemplateBatchPreservesSavingDirectionAndRollsBackInvalidRow(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []domain.Category{
		{ID: "retirement", Name: "Retirement", Kind: domain.KindExpense},
		{ID: "invalid", Name: "Invalid", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	proposals := []budgeting.BudgetProposal{
		{Category: domain.Category{ID: "retirement", Name: "Retirement"}, Bucket: budgeting.BucketSavings, LimitMinor: 10_000},
		{Category: domain.Category{ID: "invalid", Name: ""}, Bucket: budgeting.BucketNeeds, LimitMinor: 5_000},
	}
	if _, err := applyBudgetTemplate(app, proposals, nil, "USD", now); err == nil {
		t.Fatal("invalid proposal unexpectedly committed")
	}
	if got := app.Budgets(); len(got) != 0 {
		t.Fatalf("template batch partially wrote budgets: %+v", got)
	}
	count, err := applyBudgetTemplate(app, proposals[:1], nil, "USD", now)
	if err != nil || count != 1 {
		t.Fatalf("valid template write count=%d err=%v", count, err)
	}
	if got := app.Budgets(); len(got) != 1 || !got[0].IsSaving() {
		t.Fatalf("template saving budget = %+v, want one saving-direction budget", got)
	}
}

func ratesUSD() currency.Rates { return currency.Rates{Base: "USD"} }
