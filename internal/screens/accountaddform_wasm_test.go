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

// inputWithPlaceholder returns the form's input whose placeholder is text, or nil when it is not
// rendered. The debt-only fields are found this way, so a test can see them appear and disappear.
func inputWithPlaceholder(f *render.Fixture, text string) *render.QueryNode {
	for _, n := range f.AllByTag("input") {
		if n.Attr("placeholder") == text {
			return n
		}
	}
	return nil
}

// TestAccountAddFormDebtFieldsFollowTheType checks that the credit limit appears when the type is a
// credit card, and leaves when the type changes back to checking.
func TestAccountAddFormDebtFieldsFollowTheType(t *testing.T) {
	primaryAccountsGoalsApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	limit := uistate.T("accounts.creditLimit")
	if inputWithPlaceholder(f, limit) != nil {
		t.Fatal("a checking account shows the debt-only credit limit field")
	}
	byTestID(f, "select", "account-type-select").Dispatch("onchange", render.Event{Value: string(domain.TypeCreditCard)})
	f.Stabilize()
	if inputWithPlaceholder(f, limit) == nil {
		t.Fatal("choosing a credit card did not show the credit limit field")
	}
	byTestID(f, "select", "account-type-select").Dispatch("onchange", render.Event{Value: string(domain.TypeChecking)})
	f.Stabilize()
	if inputWithPlaceholder(f, limit) != nil {
		t.Fatal("switching back to checking left the debt-only field showing")
	}
}

// TestAccountAddFormCopiesAnExistingAccount checks that copying an account fills the draft with its
// copy name, so saving the form adds that copy.
func TestAccountAddFormCopiesAnExistingAccount(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	source := domain.Account{ID: "copy-source", Name: "Source Account", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD"), BalanceAsOf: time.Now()}
	if err := app.PutAccount(source); err != nil {
		t.Fatal(err)
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	byTestID(f, "select", "account-copy-existing").Dispatch("onchange", render.Event{Value: source.ID})
	f.Stabilize()
	byTestID(f, "form", "account-add-form").Submit()
	f.Stabilize()

	want := uistate.T("budgets.copySuffix", source.Name)
	for _, a := range app.Accounts() {
		if a.Name == want {
			return
		}
	}
	t.Fatalf("copying %q and saving did not add an account named %q", source.Name, want)
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
