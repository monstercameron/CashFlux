// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

var r1jNotice state.Atom[uistate.Notice]

func payeeCleanNoticeTestShell(_ struct{}) ui.Node {
	r1jNotice = uistate.UseNotice()
	return Fragment()
}

func TestPayeeCleanSaveConfirmsTheSavedName(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	if err := app.PutAccount(domain.Account{
		ID: "acct-r1j", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "txn-r1j", AccountID: "acct-r1j", Date: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		Payee: "APLPAY GREENFIELD MKT", Desc: "APLPAY GREENFIELD MKT", Amount: money.New(-1200, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })

	f := render.New(t)
	f.Render(ui.CreateElement(payeeCleanNoticeTestShell, struct{}{}))
	if err := savePayeeClean(app, domain.Transaction{ID: "txn-r1j", Payee: "APLPAY GREENFIELD MKT"},
		"APLPAY GREENFIELD MKT", "Greenfield Mkt", "all", true); err != nil {
		t.Fatalf("savePayeeClean: %v", err)
	}

	if got := r1jNotice.Get().Text; got != "Merchant name saved as Greenfield Mkt." {
		t.Errorf("save confirmation = %q, want confirmation naming the saved merchant", got)
	}
	if got := app.PayeeAliases(); len(got) != 1 || got[0].Display != "Greenfield Mkt" {
		t.Errorf("saved aliases = %#v, want the confirmed display name", got)
	}
}
