// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBQ10AStableBudgetsHostWhenDialogsToggle(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(bq10aBudgetsHost, struct{}{}))

	for _, id := range []string{
		"budgets:monthCloseOpen",
		"budgets:coverAllOpen",
		"budgets:trackUntrackedOpen",
		"budgets:sweep-config-open",
	} {
		atom := state.NewGlobalAtom(id, false)
		atom.Set(true)
		f.Stabilize()
		atom.Set(false)
		f.Stabilize()
	}
}

func bq10aBudgetsHost(_ struct{}) ui.Node { return Budgets() }

func TestBQ10ACoverAllKeepsViewedPeriodAndSafeDonorOrder(t *testing.T) {
	app := guardTestApp(t)
	for _, category := range []domain.Category{
		{ID: "bq-over-cat", Name: "Dining", Kind: domain.KindExpense},
		{ID: "bq-near-cat", Name: "Shopping", Kind: domain.KindExpense},
		{ID: "bq-safe-cat", Name: "Utilities", Kind: domain.KindExpense},
		{ID: "bq-safe2-cat", Name: "Travel", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutAccount(domain.Account{ID: "bq-account", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", BalanceAsOf: time.Now()}); err != nil {
		t.Fatal(err)
	}
	over := domain.Budget{ID: "bq-over", Name: "Dining", CategoryID: "bq-over-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(10000, "USD"), Rollover: true,
		RolloverStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	near := domain.Budget{ID: "bq-near", Name: "Shopping", CategoryID: "bq-near-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}
	safe := domain.Budget{ID: "bq-safe", Name: "Utilities", CategoryID: "bq-safe-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}
	safest := domain.Budget{ID: "bq-safest", Name: "Travel", CategoryID: "bq-safe2-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(200000, "USD")}
	for _, budget := range []domain.Budget{over, near, safe, safest} {
		if err := app.PutBudget(budget); err != nil {
			t.Fatal(err)
		}
	}
	for _, tx := range []domain.Transaction{
		{ID: "bq-over-tx", AccountID: "bq-account", Desc: "Dining", Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), CategoryID: over.CategoryID, Amount: money.New(-12000, "USD")},
		{ID: "bq-near-tx", AccountID: "bq-account", Desc: "Shopping", Date: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), CategoryID: near.CategoryID, Amount: money.New(-9000, "USD")},
	} {
		if err := app.PutTransaction(tx); err != nil {
			t.Fatal(err)
		}
	}
	viewed := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	window := period.NewWindow(period.Month, viewed, time.Monday)
	pr := prefs.Prefs{WeekStart: prefs.WeekMonday}
	overs, donors, _ := coverAllData(app, "", window, pr, true)
	if len(overs) != 1 || overs[0].ID != over.ID || !overs[0].ThisStart.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("cover-all periods = %+v; want only the October overage", overs)
	}
	if len(donors) != 2 || donors[0].ID != safest.ID || donors[1].ID != safe.ID {
		t.Fatalf("donors = %+v; want safe budgets ranked by most room", donors)
	}
	if got := coverAllDefaultSource(overs[0], donors); got != safest.ID {
		t.Fatalf("default donor = %q, want safest budget %q (never next period)", got, safest.ID)
	}
	for _, donor := range donors {
		if donor.ID == near.ID {
			t.Fatal("near-limit budget was offered as a safe donor")
		}
	}
}
