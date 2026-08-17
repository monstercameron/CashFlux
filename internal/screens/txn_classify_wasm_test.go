// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The movement classifier in the "Edit this transaction" modal — the control that
// lets an imported row become a transfer instead of phantom income or spending.
//
// The rules it applies are tested natively in internal/txnclassify. What can only
// be tested here is the control itself: which options it offers, what it says the
// save will do, when the debt claim appears, and that both handlers report the
// value they showed. A picker that offers the right choices and reports the wrong
// id would silently misfile money, and no pure test can see that.

func classifyTestApp(t *testing.T) *appstate.App {
	t.Helper()
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	accs := []domain.Account{
		{ID: "chk", Name: "SCCU Checkings", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
			OpeningBalance: money.New(500000, "USD")},
		{ID: "sav", Name: "SCCU Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
			OpeningBalance: money.New(0, "USD")},
		{ID: "card", Name: "Apple Credit Card", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
			OpeningBalance: money.New(100000, "USD")},
	}
	for _, acc := range accs {
		if err := a.PutAccount(acc); err != nil {
			t.Fatalf("PutAccount(%s): %v", acc.Name, err)
		}
	}
	return a
}

func classifyTestTxn() domain.Transaction {
	return domain.Transaction{
		ID: "imp1", AccountID: "chk", Desc: "Transfer to Savings *6500",
		Amount: money.New(-50000, "USD"),
	}
}

// testCheckHandler wraps a plain callback as the checkbox handler the form
// normally supplies from ui.UseEvent. WrapHandler leaks a js.Func per call and
// is wrong inside a render body for that reason; a test makes a bounded handful,
// and there is no hook slot available outside a component.
func testCheckHandler(fn func(bool)) ui.Handler {
	return ui.WrapHandler(func(e ui.Event) { fn(e.IsChecked()) })
}

// noopCheckHandler is the handler for cases that must never fire one; firing is
// itself the failure.
func noopCheckHandler(t *testing.T) ui.Handler {
	t.Helper()
	return testCheckHandler(func(bool) { t.Errorf("the debt claim handler fired unexpectedly") })
}

// findByTestID returns the first rendered node carrying data-testid, or nil.
func findByTestID(f *render.Fixture, tag, testID string) *render.QueryNode {
	for _, n := range f.AllByTag(tag) {
		if n.Attr("data-testid") == testID {
			return n
		}
	}
	return nil
}

// prop reads a DOM PROPERTY as a string. value/selected/checked are set as
// properties by the html shorthand (PropOption), not attributes, so Attr() reads
// empty for all three — a trap worth naming once here rather than per assertion.
func prop(n *render.QueryNode, name string) string {
	return fmt.Sprint(n.Property(name))
}

func TestClassifyFieldOffersEveryOtherAccountAndAWayOut(t *testing.T) {
	app := classifyTestApp(t)
	f := render.New(t)
	f.Render(classifyField(app, classifyTestTxn(), "", false, func(string) {}, noopCheckHandler(t)))

	sel := findByTestID(f, "select", "txn-edit-classify")
	if sel == nil {
		t.Fatal("the classifier picker did not render")
	}
	var labels, values []string
	for _, o := range f.AllByTag("option") {
		labels = append(labels, strings.TrimSpace(o.Text()))
		values = append(values, prop(o, "value"))
	}
	if len(values) != 3 {
		t.Fatalf("got %d options %v, want 3 (none + the two other accounts)", len(values), labels)
	}
	if values[0] != "" {
		t.Errorf("option 0 value = %q, want the empty way out", values[0])
	}
	if !strings.Contains(labels[0], "income or spending") {
		t.Errorf("the way out reads %q — it must name what the row stays", labels[0])
	}
	// Which accounts are offered is this control's contract; the ORDER is
	// app.Accounts()' (it sorts), and txnclassify.Counterparties is separately
	// tested for preserving whatever order it is handed.
	byValue := map[string]string{}
	for i, v := range values {
		byValue[v] = labels[i]
	}
	if _, offered := byValue["chk"]; offered {
		t.Errorf("the row's own account was offered as a counterparty")
	}
	if _, offered := byValue["sav"]; !offered {
		t.Errorf("the savings account was not offered; got %v", values)
	}
	if got := byValue["card"]; got != "Apple Credit Card" {
		t.Errorf("the card option reads %q, want its display name", got)
	}
}

func TestClassifyFieldReportsTheAccountItShows(t *testing.T) {
	app := classifyTestApp(t)
	got, calls := "", 0
	f := render.New(t)
	f.Render(classifyField(app, classifyTestTxn(), "", false,
		func(v string) { got, calls = v, calls+1 }, noopCheckHandler(t)))

	sel := findByTestID(f, "select", "txn-edit-classify")
	if sel == nil {
		t.Fatal("the classifier picker did not render")
	}
	sel.Change("card")
	if calls != 1 {
		t.Fatalf("OnChange fired %d times, want 1", calls)
	}
	if got != "card" {
		t.Errorf("OnChange reported %q, want the account id it showed", got)
	}
}

// Choosing a card or loan is what turns a transfer into a debt payment, so that
// is the only case where the claim is offered.
func TestClassifyFieldOffersTheDebtClaimOnlyForADebt(t *testing.T) {
	app := classifyTestApp(t)
	f := render.New(t)

	cases := []struct {
		name       string
		selected   string
		wantClaim  bool
		wantEffect string
	}{
		{"nothing chosen", "", false, ""},
		{"an asset", "sav", false, "between your own accounts"},
		{"a debt", "card", true, "pays down what you owe on Apple Credit Card"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f.Render(classifyField(app, classifyTestTxn(), c.selected, false,
				func(string) {}, noopCheckHandler(t)))

			claim := findByTestID(f, "input", "txn-edit-classify-debt")
			if c.wantClaim && claim == nil {
				t.Fatalf("no debt claim offered for a card")
			}
			if !c.wantClaim && claim != nil {
				t.Fatalf("a debt claim was offered for %q", c.selected)
			}
			if c.wantClaim {
				if got := claim.Attr("aria-label"); !strings.Contains(got, "Apple Credit Card") {
					t.Errorf("claim label = %q, want it to name the debt", got)
				}
			}

			effect := findByTestID(f, "p", "txn-edit-classify-effect")
			if c.wantEffect == "" {
				if effect != nil {
					t.Errorf("an effect was stated with nothing chosen: %q", effect.Text())
				}
				return
			}
			if effect == nil {
				t.Fatalf("no effect stated for %q", c.selected)
			}
			if !strings.Contains(effect.Text(), c.wantEffect) {
				t.Errorf("effect = %q, want it to contain %q", effect.Text(), c.wantEffect)
			}
		})
	}
}

func TestClassifyFieldDebtClaimReflectsAndReportsItsState(t *testing.T) {
	app := classifyTestApp(t)
	checked, calls := false, 0
	f := render.New(t)

	// Unticked: the box renders unchecked.
	f.Render(classifyField(app, classifyTestTxn(), "card", false,
		func(string) {}, testCheckHandler(func(v bool) { checked, calls = v, calls+1 })))
	claim := findByTestID(f, "input", "txn-edit-classify-debt")
	if claim == nil {
		t.Fatal("the debt claim did not render")
	}
	if prop(claim, "checked") == "true" {
		t.Errorf("claim rendered ticked when it is not")
	}
	claim.Dispatch("onchange", render.Event{Checked: true})
	if calls != 1 || !checked {
		t.Errorf("ticking reported calls=%d checked=%v, want 1 and true", calls, checked)
	}

	// Ticked: the box renders checked, so reopening the modal shows the truth.
	f.Render(classifyField(app, classifyTestTxn(), "card", true,
		func(string) {}, testCheckHandler(func(bool) {})))
	claim = findByTestID(f, "input", "txn-edit-classify-debt")
	if claim == nil {
		t.Fatal("the debt claim did not render")
	}
	if prop(claim, "checked") != "true" {
		t.Errorf("a ticked claim rendered unticked (checked=%q)", prop(claim, "checked"))
	}
}

// An already-classified row opens with its counterparty selected, so the modal
// tells the truth about what the row currently is.
func TestClassifyFieldSeedsFromTheStoredRow(t *testing.T) {
	app := classifyTestApp(t)
	txn := classifyTestTxn()
	txn.TransferAccountID = "sav"
	f := render.New(t)
	f.Render(classifyField(app, txn, txn.TransferAccountID, false, func(string) {}, noopCheckHandler(t)))

	sel := findByTestID(f, "select", "txn-edit-classify")
	if sel == nil {
		t.Fatal("the classifier picker did not render")
	}
	var selected string
	for _, o := range f.AllByTag("option") {
		if prop(o, "selected") == "true" {
			selected = prop(o, "value")
		}
	}
	if selected != "sav" {
		t.Errorf("selected option = %q, want sav — the modal must open on what the row already is", selected)
	}
}

// With no second account there is no question to ask, so nothing is drawn rather
// than a picker whose only answer is "none".
func TestClassifyFieldSaysNothingWithOnlyOneAccount(t *testing.T) {
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	if err := a.PutAccount(domain.Account{
		ID: "chk", Name: "Only Account", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	f := render.New(t)
	f.Render(classifyField(a, classifyTestTxn(), "", false, func(string) {}, noopCheckHandler(t)))

	if findByTestID(f, "select", "txn-edit-classify") != nil {
		t.Errorf("a picker was drawn with no other account to pick")
	}
	if n := findByTestID(f, "div", "txn-edit-classify-field"); n != nil {
		t.Errorf("the classifier field was drawn with nothing to offer")
	}
}

// The hint states the consequence in the user's terms — that balances do not
// move — because the control's whole risk is reading as "this moves my money".
func TestClassifyFieldSaysBalancesDoNotChange(t *testing.T) {
	app := classifyTestApp(t)
	f := render.New(t)
	f.Render(classifyField(app, classifyTestTxn(), "", false, func(string) {}, noopCheckHandler(t)))
	if got := f.Text(); !strings.Contains(got, "Balances do not change") {
		t.Errorf("the classifier never says balances are unaffected:\n%s", got)
	}
}
