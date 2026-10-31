// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/services"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestAccountsServicesViewStaysOnWhenThereAreNoServices checks that choosing Services with no services
// yet shows an empty state rather than switching back to the asset list.
func TestAccountsServicesViewStaysOnWhenThereAreNoServices(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	lens := state.NewGlobalAtom("household:activeScope", scope.ReportScope{})
	previousLens := lens.Get()
	t.Cleanup(func() { lens.Set(previousLens); uistate.PersistAcctClass(""); clearAccountDetailCaches() })
	lens.Set(scope.ReportScope{})

	now := time.Now()
	checking := domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD"), BalanceAsOf: now}
	card := domain.Account{ID: "card", Name: "Card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(-5000, "USD"), BalanceAsOf: now}
	for _, a := range []domain.Account{checking, card} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}

	uistate.PersistAcctClass(serviceClassView)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Accounts))
	f.Stabilize()

	if byTestID(f, "p", "acct-services-none") == nil {
		t.Fatalf("Services view with no services should show its empty state: %s", f.Text())
	}
	if byTestID(f, "div", "acct-row-checking") != nil {
		t.Fatal("Services view fell back to the account list")
	}
}

// TestAccountsServicesViewListsServicesWithTheirMonthlyTotal checks the Services filter: it lists
// the recurring services instead of accounts, shows their monthly total, and leaves the accounts out.
func TestAccountsServicesViewListsServicesWithTheirMonthlyTotal(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	filters := state.NewGlobalAtom("accounts:filter", uistate.AccountsFilter{})
	previousFilter := filters.Get()
	lens := state.NewGlobalAtom("household:activeScope", scope.ReportScope{})
	previousLens := lens.Get()
	t.Cleanup(func() { filters.Set(previousFilter); lens.Set(previousLens); clearAccountDetailCaches() })
	lens.Set(scope.ReportScope{})

	now := time.Now()
	checking := domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD"), BalanceAsOf: now}
	if err := app.PutAccount(checking); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutRecurring(services.NewMonthly(services.Draft{ID: "xfinity", Name: "Comcast Xfinity", Provider: "Comcast",
		Currency: "USD", MonthlyMinor: 8000, NextDue: now.AddDate(0, 0, 3)})); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutRecurring(services.NewMonthly(services.Draft{ID: "power", Name: "Power",
		Currency: "USD", MonthlyMinor: 5000, NextDue: now.AddDate(0, 0, 9)})); err != nil {
		t.Fatal(err)
	}

	// The class view starts from the saved preference, so select the Services view through it.
	uistate.PersistAcctClass(serviceClassView)
	t.Cleanup(func() { uistate.PersistAcctClass("") })
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Accounts))
	f.Stabilize()

	total := byTestID(f, "p", "acct-services-total")
	if total == nil || !strings.Contains(total.Text(), "130.00") {
		t.Fatalf("services total = %v, want 130.00 for 80.00 and 50.00 a month; page: %s", total, f.Text())
	}
	if byTestID(f, "div", "acct-service-xfinity") == nil || byTestID(f, "div", "acct-service-power") == nil {
		t.Fatalf("services view did not list both services: %s", f.Text())
	}
	if byTestID(f, "div", "acct-row-checking") != nil {
		t.Fatal("services view still lists the accounts")
	}
}
