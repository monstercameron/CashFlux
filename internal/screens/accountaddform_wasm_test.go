// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/accountedit"
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
	source := checkingFixture("copy-source", "Source Account", 10000)
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

// accountNamed returns the saved account with the name, or false when there is none.
func accountNamed(app interface{ Accounts() []domain.Account }, name string) (domain.Account, bool) {
	for _, a := range app.Accounts() {
		if a.Name == name {
			return a, true
		}
	}
	return domain.Account{}, false
}

// inputWithAriaLabel returns the input whose aria-label is label, or nil when it is not rendered.
func inputWithAriaLabel(f *render.Fixture, label string) *render.QueryNode {
	for _, n := range f.AllByTag("input") {
		if n.Attr("aria-label") == label {
			return n
		}
	}
	return nil
}

// TestAccountAddFormSavesALiability checks a debt: the credit limit and APR typed into the debt-only
// fields are saved on a liability account of the chosen type.
func TestAccountAddFormSavesALiability(t *testing.T) {
	app := primaryAccountsGoalsApp(t)

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{PrefillLiability: true}))
	f.Stabilize()

	byTestID(f, "input", "account-add-name").Input("Travel Card")
	f.Stabilize()
	inputWithPlaceholder(f, uistate.T("accounts.creditLimit")).Input("5000")
	f.Stabilize()
	inputWithPlaceholder(f, uistate.T("accounts.apr")).Input("19.9")
	f.Stabilize()
	byTestID(f, "form", "account-add-form").Submit()
	f.Stabilize()

	saved, ok := accountNamed(app, "Travel Card")
	if !ok {
		t.Fatalf("the liability was not saved: %s", f.Text())
	}
	if saved.Class != domain.ClassLiability || saved.Type != domain.TypeCreditCard {
		t.Fatalf("saved class/type = %q/%q, want liability/credit card", saved.Class, saved.Type)
	}
	if saved.CreditLimit.Amount != 500000 {
		t.Fatalf("credit limit = %d minor units, want 500000", saved.CreditLimit.Amount)
	}
	if apr, recorded := saved.RateAPR(); !recorded || apr != 19.9 {
		t.Fatalf("APR = %v, %v, want 19.9 recorded", apr, recorded)
	}
}

// TestAccountAddFormEnforcesOwnershipSharesTotal checks the split-ownership rule: while the split is
// open the shares must total 100, with no shares at all counting as a total of 0. The alert shows the
// current total, the save is refused until the shares total 100, and a figure that is not a whole
// percentage leaves the previous share in place.
func TestAccountAddFormEnforcesOwnershipSharesTotal(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	for _, m := range []domain.Member{{ID: "m-ana", Name: "Ana"}, {ID: "m-ben", Name: "Ben"}} {
		if err := app.PutMember(m); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	submit := func() {
		byTestID(f, "form", "account-add-form").Submit()
		f.Stabilize()
	}
	alertShows := func(sum int) bool {
		alert := byTestID(f, "p", "account-share-sum-error")
		return alert != nil && strings.Contains(alert.Text(), uistate.T("account.shareSumError", sum))
	}
	byTestID(f, "input", "account-add-name").Input("Joint Savings")
	f.Stabilize()
	byTestID(f, "button", "account-split-toggle").Click()
	f.Stabilize()

	// No shares entered: the alert shows a total of 0 and the save is refused.
	if !alertShows(0) {
		t.Fatalf("split with no shares should alert a total of 0: %s", f.Text())
	}
	submit()
	if _, saved := accountNamed(app, "Joint Savings"); saved {
		t.Fatal("a split with no shares was saved")
	}

	share := func(member, value string) {
		inputWithAriaLabel(f, member+" "+uistate.T("account.sharePercent")).Input(value)
		f.Stabilize()
	}
	share("Ana", "60")
	share("Ben", "30")
	if !alertShows(90) {
		t.Fatalf("shares of 60 and 30 should alert a total of 90: %s", f.Text())
	}
	// A figure that is not a whole percentage is rejected, not turned into 0.
	share("Ben", "12.5")
	if !alertShows(90) {
		t.Fatalf("a rejected figure should leave the total at 90: %s", f.Text())
	}
	submit()
	if _, saved := accountNamed(app, "Joint Savings"); saved {
		t.Fatal("shares totalling 90 were saved")
	}

	share("Ben", "40")
	if byTestID(f, "p", "account-share-sum-error") != nil {
		t.Fatalf("shares totalling 100 still show the alert: %s", f.Text())
	}
	submit()
	saved, ok := accountNamed(app, "Joint Savings")
	if !ok {
		t.Fatalf("shares totalling 100 were not saved: %s", f.Text())
	}
	if saved.OwnershipShares["m-ana"] != 60 || saved.OwnershipShares["m-ben"] != 40 {
		t.Fatalf("saved shares = %v, want Ana 60 and Ben 40", saved.OwnershipShares)
	}
}

// TestAccountAddFormCopyCarriesTheSourceCurrency checks that copying a foreign-currency account saves
// the copy in that currency, and that copying a base-currency account afterwards saves it in the base
// currency instead of keeping the foreign one.
func TestAccountAddFormCopyCarriesTheSourceCurrency(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	baseCur := accountedit.BaseCurrencyOrDefault(app.Settings().BaseCurrency)
	euro := checkingFixture("copy-euro", "Euro Account", 10000)
	euro.Currency, euro.OpeningBalance = "EUR", money.New(10000, "EUR")
	base := checkingFixture("copy-base", "Base Account", 10000)
	base.Currency, base.OpeningBalance = baseCur, money.New(10000, baseCur)
	for _, a := range []domain.Account{euro, base} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	copyAndSave := func(source domain.Account) domain.Account {
		t.Helper()
		byTestID(f, "select", "account-copy-existing").Dispatch("onchange", render.Event{Value: source.ID})
		f.Stabilize()
		if byTestID(f, "select", "account-currency-select") == nil {
			t.Fatalf("copying %q left the currency picker hidden: %s", source.Name, f.Text())
		}
		byTestID(f, "form", "account-add-form").Submit()
		f.Stabilize()
		saved, ok := accountNamed(app, uistate.T("budgets.copySuffix", source.Name))
		if !ok {
			t.Fatalf("the copy of %q was not saved: %s", source.Name, f.Text())
		}
		return saved
	}
	if got := copyAndSave(euro); got.Currency != "EUR" {
		t.Fatalf("copy of the EUR account saved in %q, want EUR", got.Currency)
	}
	if got := copyAndSave(base); got.Currency != baseCur {
		t.Fatalf("copy of the %s account saved in %q after a foreign copy, want %s", baseCur, got.Currency, baseCur)
	}
}

// TestAccountAddFormLockUntilAppearsWithTheType checks the lock-until date that comes and goes with
// the account type: it is absent for checking, present for savings, and the form keeps working
// (its hooks stay in place) when it appears, so a savings account with a lock date can be saved.
func TestAccountAddFormLockUntilAppearsWithTheType(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	lockLabel := uistate.T("accounts.lockUntil")
	if inputWithAriaLabel(f, lockLabel) != nil {
		t.Fatal("a checking account shows the lock-until date before Advanced is open")
	}
	byTestID(f, "select", "account-type-select").Dispatch("onchange", render.Event{Value: string(domain.TypeSavings)})
	f.Stabilize()
	lock := inputWithAriaLabel(f, lockLabel)
	if lock == nil {
		t.Fatalf("choosing savings did not show the lock-until date: %s", f.Text())
	}
	byTestID(f, "input", "account-add-name").Input("Locked Savings")
	f.Stabilize()
	inputWithAriaLabel(f, lockLabel).Input("2031-06-30")
	f.Stabilize()
	byTestID(f, "form", "account-add-form").Submit()
	f.Stabilize()

	saved, ok := accountNamed(app, "Locked Savings")
	if !ok {
		t.Fatalf("the savings account with a lock date was not saved: %s", f.Text())
	}
	if got := saved.LockUntil.Format("2006-01-02"); got != "2031-06-30" {
		t.Fatalf("lock until = %q, want 2031-06-30", got)
	}
}

// TestAccountAddFormShowsEveryProblemAndClearsItOnceFixed checks a refused save: an out-of-range
// figure in the closed Advanced section opens the section and names the field, and the message goes
// away when the field is fixed.
func TestAccountAddFormShowsEveryProblemAndClearsItOnceFixed(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	byTestID(f, "input", "account-add-name").Input("Brokerage")
	f.Stabilize()
	byTestID(f, "button", "account-add-advanced").Click()
	f.Stabilize()
	inputWithPlaceholder(f, uistate.T("accounts.liquidity")).Input("500")
	f.Stabilize()
	byTestID(f, "button", "account-add-advanced").Click()
	f.Stabilize()
	if inputWithPlaceholder(f, uistate.T("accounts.liquidity")) != nil {
		t.Fatal("closing Advanced left the liquidity field showing")
	}

	byTestID(f, "form", "account-add-form").Submit()
	f.Stabilize()
	want := uistate.T("accounts.fieldRangeBetween", uistate.T("accounts.field.liquidity"), 0, 100)
	if !strings.Contains(f.Text(), want) {
		t.Fatalf("the refused save does not name the liquidity field: %s", f.Text())
	}
	liquidity := inputWithPlaceholder(f, uistate.T("accounts.liquidity"))
	if liquidity == nil {
		t.Fatal("the refused save left Advanced closed, hiding the field in error")
	}
	if liquidity.Attr("aria-invalid") != "true" {
		t.Fatal("the field in error is not marked invalid")
	}
	if _, saved := accountNamed(app, "Brokerage"); saved {
		t.Fatal("an out-of-range liquidity score was saved")
	}

	liquidity.Input("80")
	f.Stabilize()
	if strings.Contains(f.Text(), want) {
		t.Fatalf("the message stayed after the field was fixed: %s", f.Text())
	}
}

// TestAccountAddFormMarksOnlyTheNameInvalidForANameProblem checks that the name input is marked
// invalid for a missing name and the mark leaves once a name is typed.
func TestAccountAddFormMarksOnlyTheNameInvalidForANameProblem(t *testing.T) {
	primaryAccountsGoalsApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(AccountAddForm, AccountAddFormProps{}))
	f.Stabilize()

	byTestID(f, "form", "account-add-form").Submit()
	f.Stabilize()
	if byTestID(f, "input", "account-add-name").Attr("aria-invalid") != "true" {
		t.Fatal("a missing name did not mark the name invalid")
	}
	byTestID(f, "input", "account-add-name").Input("Rainy Day")
	f.Stabilize()
	if got := byTestID(f, "input", "account-add-name").Attr("aria-invalid"); got == "true" {
		t.Fatal("the name stayed marked invalid after a name was typed")
	}
	if strings.Contains(f.Text(), uistate.T("accounts.nameRequired")) {
		t.Fatalf("the required-name message stayed after a name was typed: %s", f.Text())
	}
}
