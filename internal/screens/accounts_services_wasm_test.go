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

// checkingFixture is a shared checking account with an opening balance, as the Accounts tests need.
func checkingFixture(id, name string, openingMinor int64) domain.Account {
	return domain.Account{ID: id, Name: name, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(openingMinor, "USD"), BalanceAsOf: time.Now()}
}

// isolateAccountsView gives a test a clean Accounts view: no household lens, the default account
// filter and class view, and no cached account details. One cleanup puts all of it back.
func isolateAccountsView(t *testing.T) {
	t.Helper()
	filters := state.NewGlobalAtom("accounts:filter", uistate.AccountsFilter{})
	lens := state.NewGlobalAtom("household:activeScope", scope.ReportScope{})
	previousFilter, previousLens := filters.Get(), lens.Get()
	t.Cleanup(func() {
		filters.Set(previousFilter)
		lens.Set(previousLens)
		uistate.PersistAcctClass("")
		clearAccountDetailCaches()
	})
	lens.Set(scope.ReportScope{})
}

// TestAccountsServicesViewStaysOnWhenThereAreNoServices checks that choosing Services with no services
// yet shows an empty state rather than switching back to the asset list.
func TestAccountsServicesViewStaysOnWhenThereAreNoServices(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	isolateAccountsView(t)

	checking := checkingFixture("checking", "Checking", 10000)
	card := domain.Account{ID: "card", Name: "Card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(-5000, "USD"), BalanceAsOf: time.Now()}
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
	isolateAccountsView(t)

	now := time.Now()
	if err := app.PutAccount(checkingFixture("checking", "Checking", 10000)); err != nil {
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
