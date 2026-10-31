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

// TestAccountEditFormRefusesAnUnreadableOpeningBalance checks that a figure the account builder
// rejects shows its message under the form even when the account has no custom fields, and that
// nothing is saved.
func TestAccountEditFormRefusesAnUnreadableOpeningBalance(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 10000)
	if err := app.PutAccount(checking); err != nil {
		t.Fatal(err)
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{AccountID: checking.ID, Mode: uistate.AcctEditModeEdit}))
	f.Stabilize()

	inputWithPlaceholder(f, uistate.T("accounts.openingBalance")).Input("twelve")
	f.Stabilize()
	f.AllByTag("form")[0].Submit()
	f.Stabilize()

	if !strings.Contains(f.Text(), uistate.T("accounts.invalidOpening")) {
		t.Fatalf("the refused save shows no message: %s", f.Text())
	}
	var saved domain.Account
	for _, a := range app.Accounts() {
		if a.ID == checking.ID {
			saved = a
		}
	}
	if saved.OpeningBalance.Amount != 10000 {
		t.Fatalf("opening balance = %d after a refused save, want 10000", saved.OpeningBalance.Amount)
	}
}

// TestAccountEditFormSavesAValidEdit checks the happy path of the same form: a renamed account is
// saved and the modal closes.
func TestAccountEditFormSavesAValidEdit(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 10000)
	if err := app.PutAccount(checking); err != nil {
		t.Fatal(err)
	}
	closed := 0
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{
		AccountID: checking.ID, Mode: uistate.AcctEditModeEdit, OnDone: func() { closed++ },
	}))
	f.Stabilize()

	inputWithID(f, "acct-edit-"+checking.ID).Input("Everyday")
	f.Stabilize()
	f.AllByTag("form")[0].Submit()
	f.Stabilize()

	for _, a := range app.Accounts() {
		if a.ID == checking.ID && a.Name != "Everyday" {
			t.Fatalf("account name = %q, want Everyday", a.Name)
		}
	}
	if closed != 1 {
		t.Fatalf("OnDone called %d times, want 1", closed)
	}
}

// TestAccountEditFormReconcileMarksEveryUnclearedRowCleared checks the reconcile modal's
// "Mark all cleared" action: every uncleared row of this account flips to cleared, and a row of
// another account is left alone.
func TestAccountEditFormReconcileMarksEveryUnclearedRowCleared(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 0)
	other := checkingFixture("other", "Other", 0)
	for _, a := range []domain.Account{checking, other} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	for _, txn := range []domain.Transaction{
		{ID: "t1", AccountID: checking.ID, Date: time.Now(), Desc: "Coffee", Amount: money.New(-500, "USD")},
		{ID: "t2", AccountID: checking.ID, Date: time.Now(), Desc: "Lunch", Amount: money.New(-1200, "USD")},
		{ID: "t3", AccountID: other.ID, Date: time.Now(), Desc: "Elsewhere", Amount: money.New(-900, "USD")},
	} {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{AccountID: checking.ID, Mode: uistate.AcctEditModeReconcile}))
	f.Stabilize()

	byTestID(f, "button", "reconcile-markall-btn").Click()
	f.Stabilize()

	cleared := map[string]bool{}
	for _, txn := range app.Transactions() {
		cleared[txn.ID] = txn.Cleared
	}
	if !cleared["t1"] || !cleared["t2"] {
		t.Fatalf("cleared = %v, want both of this account's rows cleared", cleared)
	}
	if cleared["t3"] {
		t.Fatal("a row of another account was cleared")
	}
}

// TestAccountEditFormReconcileAdjustmentMatchesTheStatement checks that the adjustment posts a
// cleared manual row for the difference, so the account then reconciles against the typed statement.
func TestAccountEditFormReconcileAdjustmentMatchesTheStatement(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 10000)
	if err := app.PutAccount(checking); err != nil {
		t.Fatal(err)
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{AccountID: checking.ID, Mode: uistate.AcctEditModeReconcile}))
	f.Stabilize()

	byTestID(f, "input", "reconcile-statement-input").Input("125.50")
	f.Stabilize()
	byTestID(f, "button", "reconcile-adjust").Click()
	f.Stabilize()

	var adjustment domain.Transaction
	for _, txn := range app.Transactions() {
		if txn.AccountID == checking.ID {
			adjustment = txn
		}
	}
	if adjustment.ID == "" {
		t.Fatalf("no adjustment was posted: %s", f.Text())
	}
	if adjustment.Amount.Amount != 2550 || !adjustment.Cleared || !adjustment.Reviewed || adjustment.Source != domain.TxnSourceManual {
		t.Fatalf("adjustment = %+v, want a cleared, reviewed, manual +25.50 row", adjustment)
	}
	if byTestID(f, "span", "reconcile-confirmed") == nil {
		t.Fatalf("the account does not show as reconciled after the adjustment: %s", f.Text())
	}
}

// editFormAccount returns the stored account with the id.
func editFormAccount(t *testing.T, app interface{ Accounts() []domain.Account }, id string) domain.Account {
	t.Helper()
	for _, a := range app.Accounts() {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("account %q is not stored", id)
	return domain.Account{}
}

// TestAccountEditFormRefusesAnUnreadableAsOfDate checks that a new value typed with a date that is
// not a real date is refused with a message, and that the editor stays open and nothing is saved,
// instead of the date silently becoming today.
func TestAccountEditFormRefusesAnUnreadableAsOfDate(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 10000)
	if err := app.PutAccount(checking); err != nil {
		t.Fatal(err)
	}
	before := len(app.Transactions())
	closed := 0
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{
		AccountID: checking.ID, Mode: uistate.AcctEditModeEdit, OnDone: func() { closed++ },
	}))
	f.Stabilize()

	byTestID(f, "input", "acct-setbal-input").Input("50")
	f.Stabilize()
	byTestID(f, "input", "acct-setbal-asof").Input("not-a-date")
	f.Stabilize()
	f.AllByTag("form")[0].Submit()
	f.Stabilize()

	if !strings.Contains(f.Text(), uistate.T("accounts.setBalanceAsOfInvalid")) {
		t.Fatalf("the refused save shows no message about the date: %s", f.Text())
	}
	if closed != 0 {
		t.Fatalf("the editor closed %d times on a refused save", closed)
	}
	if n := len(app.Transactions()); n != before {
		t.Fatalf("%d transactions after a refused save, want the %d it started with", n, before)
	}
}

// TestAccountEditFormShowsAVariableNameClashWhereItIsFixed checks that a variable name another
// account already uses refuses the save with a visible message, even though the variable-name
// field sits in the Advanced section, which was closed.
func TestAccountEditFormShowsAVariableNameClashWhereItIsFixed(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 10000)
	other := checkingFixture("other", "Other", 0)
	for _, a := range []domain.Account{checking, other} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	closed := 0
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{
		AccountID: checking.ID, Mode: uistate.AcctEditModeEdit, OnDone: func() { closed++ },
	}))
	f.Stabilize()

	inputWithID(f, "acct-edit-"+checking.ID).Input("Other")
	f.Stabilize()
	f.AllByTag("form")[0].Submit()
	f.Stabilize()

	clash := entityVarCollision(accountVarKind, accountVarEntities(app.Accounts()), checking.ID, "", "Other")
	if clash == "" {
		t.Fatal("test setup: the two accounts do not share a variable name")
	}
	if !strings.Contains(f.Text(), clash) {
		t.Fatalf("the refused save shows no variable-name message: %s", f.Text())
	}
	if closed != 0 || editFormAccount(t, app, checking.ID).Name != "Checking" {
		t.Fatalf("a clashing variable name was saved or closed the editor (closed %d)", closed)
	}
}

// TestAccountEditFormReconcileWaitsForAReadableStatementBalance checks that a statement balance
// that is not a number is reported, and neither the resolution actions nor the finish button are
// offered from it.
func TestAccountEditFormReconcileWaitsForAReadableStatementBalance(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 10000)
	if err := app.PutAccount(checking); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{AccountID: checking.ID, Mode: uistate.AcctEditModeReconcile}))
	f.Stabilize()

	byTestID(f, "input", "reconcile-statement-input").Input("12x")
	f.Stabilize()

	if byTestID(f, "p", "reconcile-statement-invalid") == nil {
		t.Fatalf("an unreadable statement balance is not reported: %s", f.Text())
	}
	if byTestID(f, "div", "reconcile-resolve") != nil || byTestID(f, "button", "reconcile-done") != nil {
		t.Fatal("the resolution actions or the finish button are offered from an unreadable balance")
	}
}
