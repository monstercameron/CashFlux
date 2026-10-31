// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestAccountsServicesViewListsUtilityAccounts checks that an account of the Utilities type shows
// under the Services filter, and that it is not in the asset list while it is.
func TestAccountsServicesViewListsUtilityAccounts(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	isolateAccountsView(t)

	utility := domain.Account{ID: "gfi", Name: "Google Fi Phone Bill", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeUtilities, Currency: "USD", OpeningBalance: money.New(0, "USD"), BalanceAsOf: time.Now()}
	for _, a := range []domain.Account{checkingFixture("checking", "Checking", 10000), utility} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}

	uistate.PersistAcctClass(serviceClassView)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Accounts))
	f.Stabilize()

	row := byTestID(f, "div", "acct-utility-gfi")
	if row == nil {
		t.Fatalf("Services view should list the utility account: %s", f.Text())
	}
	if !strings.Contains(row.Text(), "Google Fi Phone Bill") {
		t.Fatalf("utility row should name the account: %q", row.Text())
	}
	if byTestID(f, "span", "acct-utility-cost-gfi") == nil {
		t.Fatal("utility row should show its monthly cost")
	}
	if byTestID(f, "div", "acct-services-none") != nil {
		t.Fatal("a utility account is a service, so the empty state must not show")
	}
	if byTestID(f, "div", "acct-row-checking") != nil {
		t.Fatal("Services view fell back to the account list")
	}
}
