// SPDX-License-Identifier: MIT

package appstate

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBC5B5PriceCreepImpactUsesEffectiveViewAndWeekPreference(t *testing.T) {
	a := newApp(t, false)
	now := moneyDate("2026-10-14")
	a.now = func() time.Time { return now }
	if err := a.SetSettingKV("cashflux:prefs", `{"weekStart":"sunday"}`); err != nil {
		t.Fatal(err)
	}
	if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutAccount(domain.Account{ID: "joint", Name: "Joint", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	b := sharedBudget("b", "Food", "food", 20000)
	b.Period = domain.PeriodWeekly
	b.PeriodBoosts = map[string]int64{"2026-10-11": 10000}
	if err := a.store.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	if err := a.store.PutTransaction(domain.Transaction{ID: "spent", AccountID: "joint", CategoryID: "food",
		Date: moneyDate("2026-10-13"), Amount: money.New(-25000, "USD")}); err != nil {
		t.Fatal(err)
	}
	r := domain.Recurring{ID: "r", Label: "Stream", AccountID: "joint", CategoryID: "food",
		Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly}
	if err := a.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	in, err := a.BudgetViewInput(now)
	if err != nil {
		t.Fatal(err)
	}
	if in.Period.WeekStart != time.Sunday {
		t.Fatalf("view week start=%v, want Sunday from preference", in.Period.WeekStart)
	}
	impact, err := a.PriceCreepBudgetImpactChecked("r", 2000)
	if err != nil {
		t.Fatal(err)
	}
	if !impact.HasBudget || impact.LimitMinor != 30000 || impact.SpentBeforeMinor != 25000 {
		t.Fatalf("impact did not use effective budget view: %+v", impact)
	}
	if impact.Currency != "USD" || impact.DeltaMonthlyMinor != 1000 {
		t.Fatalf("impact currencies/delta = %+v", impact)
	}
}

func TestBC5B5PriceCreepFXMissReturnsErrorWithoutUnconvertedImpact(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutAccount(domain.Account{ID: "joint", Name: "Joint", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRecurring(domain.Recurring{ID: "r", Label: "Stream", AccountID: "joint", CategoryID: "food",
		Amount: money.New(-1000, "EUR"), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	impact, err := a.PriceCreepBudgetImpactChecked("r", 2000)
	if err == nil || !strings.Contains(err.Error(), "convert monthly price change") {
		t.Fatalf("missing FX rate error=%v, want conversion error", err)
	}
	if impact.HasBudget || impact.DeltaMonthlyMinor != 0 {
		t.Fatalf("FX miss leaked an unconverted value: %+v", impact)
	}
}

func TestBC5B5BudgetForRecurringUsesScopeDirectionAndSpecificity(t *testing.T) {
	a := newApp(t, false)
	for _, c := range []domain.Category{
		{ID: "parent", Name: "Parent", Kind: domain.KindExpense},
		{ID: "child", Name: "Child", Kind: domain.KindExpense, ParentID: "parent"},
	} {
		if err := a.PutCategory(c); err != nil {
			t.Fatal(err)
		}
	}
	for _, account := range []domain.Account{
		{ID: "alice", Name: "Alice", OwnerID: "alice", Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "bob", Name: "Bob", OwnerID: "bob", Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
	} {
		if err := a.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	for _, b := range []domain.Budget{
		{ID: "saving", Name: "Savings", Scope: domain.ScopeIndividual, OwnerID: "alice", CategoryID: "child", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD"), Direction: domain.DirectionSave},
		{ID: "bob-child", Name: "Bob child", Scope: domain.ScopeIndividual, OwnerID: "bob", CategoryID: "child", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
		{ID: "alice-parent", Name: "Alice parent", Scope: domain.ScopeIndividual, OwnerID: "alice", CategoryID: "parent", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
		{ID: "alice-child", Name: "Alice child", Scope: domain.ScopeIndividual, OwnerID: "alice", CategoryID: "child", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
	} {
		if err := a.store.PutBudget(b); err != nil {
			t.Fatal(err)
		}
	}
	got, ok := a.budgetForRecurring(domain.Recurring{AccountID: "alice", CategoryID: "child"})
	if !ok || got.ID != "alice-child" {
		t.Fatalf("selected budget=%+v, found=%t; want alice-child", got, ok)
	}
}

func TestBC5B5BudgetFlagTaskStoresTranslationKeysAndDedupesByIdentity(t *testing.T) {
	b := sharedBudget("b", "Food", "food", 10000)
	task := newBudgetOverTask(b, budgeting.Status{Percent: 125})
	if task.Title != "tasks.budgetOverLimit" || task.Notes != budgetOverTaskNotesKey ||
		task.Custom[taskTitleKeyField] != task.Title || task.Custom[taskNotesKeyField] != task.Notes {
		t.Fatalf("task contains display copy instead of translation keys: %+v", task)
	}
	if task.Custom[taskTitleArgsField] == nil || task.Custom[taskNotesArgsField] == nil {
		t.Fatalf("task is missing localized format arguments: %+v", task.Custom)
	}
	if !duplicateBudgetOverTask([]domain.Task{task}, "b") {
		t.Fatal("same budget's open flag was not deduplicated")
	}
	if duplicateBudgetOverTask([]domain.Task{task}, "another") {
		t.Fatal("different budget was incorrectly deduplicated")
	}
	task.Status = domain.StatusDone
	if duplicateBudgetOverTask([]domain.Task{task}, "b") {
		t.Fatal("closed task should not block a new flag")
	}
}
