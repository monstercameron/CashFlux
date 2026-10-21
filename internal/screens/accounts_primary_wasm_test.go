// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func primaryAccountsGoalsApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous; _ = app.Store().Close() })
	return app
}

func TestPrimaryAccountSnapshotTracksLedgerRevision(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	a := domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(100, "USD")}
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	before := accountSnapshot(app)
	a.OpeningBalance = money.New(200, "USD")
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	after := accountSnapshot(app)
	if before.Balances[a.ID].Amount != 100 || after.Balances[a.ID].Amount != 200 {
		t.Fatalf("stale balance snapshot: before=%v after=%v", before.Balances, after.Balances)
	}
}

func TestPrimaryAccountGroupShowsIncompleteSubtotal(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(acctGroupHeader, acctGroupHeaderProps{Section: accountview.GroupSection{
		Group: domain.AccountGroup{ID: "cash", Name: "Cash"}, Subtotal: money.New(100, "USD"), Incomplete: true, ExcludedCurrencies: []string{"GBP"},
	}}))
	note := byTestID(f, "span", "acct-group-incomplete-cash")
	if note == nil || !strings.Contains(note.Text(), "GBP") || !strings.Contains(note.Text(), "excluded") {
		t.Fatalf("incomplete subtotal lacks explanation: %v", note)
	}
}

func TestPrimaryAccountSettingsMenuExposesExistingManagementActions(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutAccount(domain.Account{ID: "bank", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.Zero("USD")}); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(acctToolbarWidget, acctToolbarProps{App: app}))
	f.Stabilize()
	trigger := byTestID(f, "button", "acct-manage-btn")
	if trigger == nil {
		t.Fatal("missing account settings trigger")
	}
	trigger.Click()
	f.Stabilize()
	if trigger = byTestID(f, "button", "acct-manage-btn"); trigger.Attr("aria-expanded") != "true" {
		t.Fatal("account settings did not stay open")
	}
	if byTestID(f, "button", "acct-groups-btn") == nil || byTestID(f, "button", "acct-institutions-btn") == nil {
		t.Fatalf("account management actions are missing: %s", f.Text())
	}
}
