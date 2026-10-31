// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/tasksuggest"
)

func budgetTaskApp(t *testing.T) (*App, time.Time, domain.Budget) {
	t.Helper()
	a := newApp(t, false)
	previousResolve := lastAutoResolve
	t.Cleanup(func() { lastAutoResolve = previousResolve; _ = a.Store().Close() })
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	if err := a.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 1}}); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"USD", "EUR"} {
		if err := a.PutAccount(domain.Account{ID: code, Name: code, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: code, BalanceAsOf: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	b := domain.Budget{ID: "food", Name: "Food", CategoryID: "food", Period: domain.PeriodMonthly,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Limit: money.New(10000, "USD")}
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	return a, now, b
}

func existingOverspentTask(t *testing.T, a *App, now time.Time, b domain.Budget) domain.Task {
	t.Helper()
	rates := currency.Rates{Base: "USD", Rates: a.Settings().FXRates}
	suggestions := tasksuggest.Scan(nil, budgeting.NewNettedLedger(a.Transactions(), a.TxnLinks()), a.Budgets(), freshness.Windows{}, rates, now, time.Sunday)
	for _, suggestion := range suggestions {
		if suggestion.Kind != tasksuggest.KindOverspentBudget || suggestion.RelatedID != b.ID {
			continue
		}
		task := domain.Task{ID: "overspent", Title: "Review Food overspending", Status: domain.StatusOpen,
			Priority: domain.PriorityMedium, Source: domain.SourceNudge, RelatedType: suggestion.RelatedType,
			RelatedID: suggestion.RelatedID, Resolve: suggestion.Resolve}
		if err := a.PutTask(task); err != nil {
			t.Fatal(err)
		}
		return task
	}
	t.Fatal("known overspending did not produce the existing task condition")
	return domain.Task{}
}

func assertBudgetTaskStatus(t *testing.T, a *App, taskID string, want domain.TaskStatus) {
	t.Helper()
	for _, task := range a.Tasks() {
		if task.ID == taskID {
			if task.Status != want {
				t.Fatalf("budget task status = %s, want %s", task.Status, want)
			}
			return
		}
	}
	t.Fatal("budget task disappeared")
}

func budgetTaskPost(t *testing.T, a *App, now time.Time, id string) {
	t.Helper()
	if err := a.PutTransaction(domain.Transaction{ID: id, AccountID: "USD", Date: now,
		Payee: "Unrelated posting", Desc: "Unrelated posting", Amount: money.New(1, "USD")}); err != nil {
		t.Fatal(err)
	}
}

func TestBudgetTaskRemainsOpenOnMissingFXAndResolvesOnceAfterRealRecovery(t *testing.T) {
	a, now, b := budgetTaskApp(t)
	if err := a.Store().PutTransaction(domain.Transaction{ID: "foreign", AccountID: "EUR", CategoryID: b.CategoryID,
		Date: now, Payee: "Food expense", Amount: money.New(-15000, "EUR")}); err != nil {
		t.Fatal(err)
	}
	task := existingOverspentTask(t, a, now, b)
	notices := 0
	a.Notifier = func(message string) {
		if strings.HasPrefix(message, "Done for you:") {
			notices++
		}
	}
	budgetTaskPost(t, a, now, "known-over")
	assertBudgetTaskStatus(t, a, task.ID, domain.StatusOpen)
	if err := a.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		budgetTaskPost(t, a, now, fmt.Sprintf("unknown-%d", i))
		assertBudgetTaskStatus(t, a, task.ID, domain.StatusOpen)
	}
	if _, found := a.engineVars()["budget_food_over"]; found || notices != 0 {
		t.Fatal("missing FX published safe overspending or completion notice")
	}
	if err := a.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 1}}); err != nil {
		t.Fatal(err)
	}
	budgetTaskPost(t, a, now, "rates-restored")
	assertBudgetTaskStatus(t, a, task.ID, domain.StatusOpen)
	// Rates being restored is insufficient: only the real refund clears the
	// overspend. This uses the existing condition and production posting hook.
	if err := a.PutTransaction(domain.Transaction{ID: "refund", AccountID: "EUR", CategoryID: b.CategoryID,
		Date: now, Payee: "Food refund", Desc: "Food refund", Amount: money.New(5000, "EUR")}); err != nil {
		t.Fatal(err)
	}
	assertBudgetTaskStatus(t, a, task.ID, domain.StatusOpen)
	if err := a.PutTxnLink(domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"foreign", "refund"}, Amount: money.New(5000, "EUR")}); err != nil {
		t.Fatal(err)
	}
	budgetTaskPost(t, a, now, "refund-linked")
	assertBudgetTaskStatus(t, a, task.ID, domain.StatusDone)
	budgetTaskPost(t, a, now, "later")
	if notices != 1 || len(a.Tasks()) != 1 {
		t.Fatalf("recovery completed more than once: notices=%d tasks=%d", notices, len(a.Tasks()))
	}
}

func TestBudgetTaskRemainsOpenOnSpendingOverflow(t *testing.T) {
	a, now, b := budgetTaskApp(t)
	spend := domain.Transaction{ID: "large", AccountID: "USD", CategoryID: b.CategoryID,
		Date: now, Desc: "Large food expense", Amount: money.New(-math.MaxInt64, "USD")}
	if err := a.Store().PutTransaction(spend); err != nil {
		t.Fatal(err)
	}
	task := existingOverspentTask(t, a, now, b)
	notices := 0
	a.Notifier = func(message string) {
		if strings.HasPrefix(message, "Done for you:") {
			notices++
		}
	}
	if err := a.PutTransaction(domain.Transaction{ID: "overflow", AccountID: "USD", CategoryID: b.CategoryID,
		Date: now, Desc: "Food expense", Amount: money.New(-1, "USD")}); err != nil {
		t.Fatal(err)
	}
	assertBudgetTaskStatus(t, a, task.ID, domain.StatusOpen)
	if _, found := a.engineVars()["budget_food_over"]; found || notices != 0 {
		t.Fatal("overflow published safe overspending or completion notice")
	}
	spend.Amount = money.New(-1000, "USD")
	if err := a.PutTransaction(spend); err != nil {
		t.Fatal(err)
	}
	budgetTaskPost(t, a, now, "corrected")
	assertBudgetTaskStatus(t, a, task.ID, domain.StatusDone)
	budgetTaskPost(t, a, now, "later")
	if notices != 1 {
		t.Fatalf("overflow recovery completed %d times, want once", notices)
	}
}

func TestBudgetTaskKnownZeroStillResolvesAndUnavailableVarsStayDiscoverable(t *testing.T) {
	a, now, b := budgetTaskApp(t)
	other := b
	other.ID, other.CategoryID, other.Limit = "other", "other", money.New(20000, "USD")
	if err := a.PutBudget(other); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutTransaction(domain.Transaction{ID: "foreign", AccountID: "EUR", CategoryID: b.CategoryID,
		Date: now, Amount: money.New(-15000, "EUR")}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	vars := a.engineVars()
	names := a.WorkflowVariableNames()
	if !sort.StringsAreSorted(names) {
		t.Fatal("variable discovery lost stable sorted order")
	}
	found := map[string]bool{}
	for _, name := range names {
		found[name] = true
	}
	bases := engineenv.BudgetVarBases(a.Budgets())
	if len(bases) != 2 || bases[0].Prefix != "budget_food_" || bases[1].Prefix != "budget_food_2_" {
		t.Fatalf("duplicate slug identities changed: %+v", bases)
	}
	for _, base := range bases {
		for _, field := range engineenv.BudgetVarFields {
			if !found[base.Prefix+field] {
				t.Errorf("unavailable %s vanished from variable discovery", base.Prefix+field)
			}
		}
	}
	if _, present := vars["budget_food_over"]; present {
		t.Fatal("discovery introduced a fake runtime value")
	}
	if value, present := vars["budget_food_2_over"]; !present || value != 0 {
		t.Fatal("clean duplicate lost its measured zero")
	}
	task := domain.Task{ID: "zero", Title: "Existing task now clear", Status: domain.StatusOpen, Priority: domain.PriorityMedium,
		Resolve: &domain.TaskResolve{Condition: "budget_food_2_over <= 0"}}
	if err := a.PutTask(task); err != nil {
		t.Fatal(err)
	}
	budgetTaskPost(t, a, now, "known-zero")
	assertBudgetTaskStatus(t, a, task.ID, domain.StatusDone)
}
