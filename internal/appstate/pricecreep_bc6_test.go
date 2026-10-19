// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBC6N1PriceCreepPreviewAndAcceptRaiseUseNamedBudgetAtomically(t *testing.T) {
	a := newApp(t, false)
	a.now = func() time.Time { return moneyDate("2026-10-19") }
	for _, category := range []domain.Category{
		{ID: "entertainment", Name: "Entertainment", Kind: domain.KindExpense},
		{ID: "streaming", Name: "Streaming", Kind: domain.KindExpense, ParentID: "entertainment"},
	} {
		if err := a.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.PutAccount(domain.Account{
		ID: "alice-checking", Name: "Checking", OwnerID: "alice", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	for _, budget := range []domain.Budget{
		{ID: "a-entertainment", Name: "Entertainment", OwnerID: "alice", Scope: domain.ScopeIndividual, CategoryID: "entertainment", Period: domain.PeriodMonthly, Limit: money.New(30000, "USD")},
		{ID: "z-streaming", Name: "Streaming", OwnerID: "alice", Scope: domain.ScopeIndividual, CategoryID: "streaming", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
		{ID: "wrong-owner", Name: "Other person's Streaming", OwnerID: "bob", Scope: domain.ScopeIndividual, CategoryID: "streaming", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")},
	} {
		if err := a.PutBudget(budget); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.PutRecurring(domain.Recurring{
		ID: "netflix", Label: "Netflix", AccountID: "alice-checking", CategoryID: "streaming",
		Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly, NextDue: a.clock(),
	}); err != nil {
		t.Fatal(err)
	}

	preview, targetID, err := a.PriceCreepBudgetImpactTarget("netflix", 1500)
	if err != nil {
		t.Fatal(err)
	}
	if targetID != "z-streaming" || preview.BudgetName != "Streaming" {
		t.Fatalf("preview target=%q impact=%+v; want Streaming/z-streaming", targetID, preview)
	}
	if err := a.AcceptPriceAndRaiseBudgetForCreep("netflix", targetID, 1500); err != nil {
		t.Fatal(err)
	}
	gotRecurring, ok := a.findRecurring("netflix")
	if !ok || gotRecurring.Amount != money.New(-1500, "USD") {
		t.Fatalf("accepted recurring=%+v found=%t", gotRecurring, ok)
	}
	for _, check := range []struct {
		id   string
		want int64
	}{
		{"a-entertainment", 30000},
		{"z-streaming", 10500},
		{"wrong-owner", 50000},
	} {
		got, ok, err := a.Store().GetBudget(check.id)
		if err != nil || !ok {
			t.Fatalf("GetBudget(%q): found=%t err=%v", check.id, ok, err)
		}
		if got.Limit.Amount != check.want {
			t.Errorf("budget %q limit=%d, want %d", check.id, got.Limit.Amount, check.want)
		}
	}
}

func TestBC6N1StalePriceCreepTargetLeavesBothRecordsUnchanged(t *testing.T) {
	a := newApp(t, false)
	for _, category := range []domain.Category{
		{ID: "parent", Name: "Parent", Kind: domain.KindExpense},
		{ID: "child", Name: "Child", Kind: domain.KindExpense, ParentID: "parent"},
	} {
		if err := a.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.PutAccount(domain.Account{ID: "checking", Name: "Checking", OwnerID: "alice", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	for _, budget := range []domain.Budget{
		{ID: "parent-budget", Name: "Parent", OwnerID: "alice", Scope: domain.ScopeIndividual, CategoryID: "parent", Period: domain.PeriodMonthly, Limit: money.New(30000, "USD")},
		{ID: "child-budget", Name: "Child", OwnerID: "alice", Scope: domain.ScopeIndividual, CategoryID: "child", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
	} {
		if err := a.PutBudget(budget); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.PutRecurring(domain.Recurring{ID: "bill", Label: "Bill", AccountID: "checking", CategoryID: "child",
		Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly, NextDue: moneyDate("2026-10-19")}); err != nil {
		t.Fatal(err)
	}
	if err := a.AcceptPriceAndRaiseBudgetForCreep("bill", "parent-budget", 2000); err == nil {
		t.Fatal("stale budget target succeeded")
	}
	r, _ := a.findRecurring("bill")
	if r.Amount != money.New(-1000, "USD") {
		t.Fatalf("recurring changed after stale-target failure: %+v", r.Amount)
	}
	for id, want := range map[string]int64{"parent-budget": 30000, "child-budget": 10000} {
		b, _, err := a.Store().GetBudget(id)
		if err != nil || b.Limit.Amount != want {
			t.Errorf("budget %q after stale-target failure: %+v err=%v, want %d", id, b.Limit, err, want)
		}
	}
}

func TestBC6N1PriceCreepTaskStoresTranslationKeys(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutRecurring(domain.Recurring{ID: "bill", Label: "Internet", Amount: money.New(-5000, "USD"), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	if err := a.CreatePriceCreepTask("bill", 4000); err != nil {
		t.Fatal(err)
	}
	tasks := a.Tasks()
	if len(tasks) != 1 {
		t.Fatalf("created %d tasks, want one", len(tasks))
	}
	task := tasks[0]
	if task.Title != "pricecreep.taskTitle" || task.Notes != "pricecreep.taskNotes" ||
		task.Custom[taskTitleKeyField] != "pricecreep.taskTitle" || task.Custom[taskNotesKeyField] != "pricecreep.taskNotes" {
		t.Fatalf("task stored display copy instead of localization keys: %+v", task)
	}
	if args, ok := task.Custom[taskTitleArgsField].([]any); !ok || len(args) != 1 || args[0] != "Internet" {
		t.Fatalf("task title args=%#v", task.Custom[taskTitleArgsField])
	}
	copy := i18n.DefaultBundle()
	if got := copy.T(i18n.English, task.Custom[taskTitleKeyField].(string), "Internet"); got != "Review the price of Internet" {
		t.Fatalf("localized task title=%q", got)
	}
	if got := copy.T(i18n.English, task.Custom[taskNotesKeyField].(string), "Internet"); got == "pricecreep.taskNotes" {
		t.Fatal("price-creep task note has no English translation")
	}
}
