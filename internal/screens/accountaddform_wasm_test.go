// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestAccountAddFormAddsAnAccount drives the add form the way a person does: it types a name and
// submits. The account must be saved, and the form must reset for the next one.
func TestAccountAddFormAddsAnAccount(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	before := len(app.Accounts())

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	byTestID(f, "input", "account-add-name").Input("Savings Vault")
	f.Stabilize()
	byTestID(f, "form", "account-add-form").Submit()
	f.Stabilize()

	accounts := app.Accounts()
	if len(accounts) != before+1 {
		t.Fatalf("accounts = %d, want %d after adding one", len(accounts), before+1)
	}
	var saved bool
	for _, a := range accounts {
		if a.Name == "Savings Vault" {
			saved = true
		}
	}
	if !saved {
		t.Fatal("the typed account name was not saved")
	}
	if got := byTestID(f, "input", "account-add-name").Attr("value"); got != "" {
		t.Fatalf("name field kept %q after a successful add, want it cleared", got)
	}
}

// TestAccountAddFormRejectsAnEmptyName checks that submitting without a name saves nothing and
// shows the required-name message.
func TestAccountAddFormRejectsAnEmptyName(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	before := len(app.Accounts())

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	byTestID(f, "form", "account-add-form").Submit()
	f.Stabilize()

	if len(app.Accounts()) != before {
		t.Fatalf("an empty name saved an account: %d accounts, want %d", len(app.Accounts()), before)
	}
	if !strings.Contains(f.Text(), uistate.T("accounts.nameRequired")) {
		t.Fatalf("the required-name message is missing: %s", f.Text())
	}
}
