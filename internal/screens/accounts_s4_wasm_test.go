// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// s4Account builds a shared account for the summary, toolbar, list and transfer tests.
func s4Account(id, name, code string, class domain.AccountClass, kind domain.AccountType, minor int64) domain.Account {
	return domain.Account{ID: id, Name: name, Currency: code, Class: class, Type: kind,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, OpeningBalance: money.New(minor, code)}
}

func s4App(t *testing.T, accounts ...domain.Account) *appstate.App {
	t.Helper()
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	t.Cleanup(func() { css.SetSink(previousCSS) })
	app := primaryAccountsGoalsApp(t)
	for _, account := range accounts {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func s4Fixture(t *testing.T) *render.Fixture {
	t.Helper()
	f := render.New(t)
	t.Cleanup(func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	})
	return f
}

// The summary's disclosures are Callouts: the partial-total warning is a status-announced warn
// callout, the figure stays in the headline, and the notice leaves with the cause.
func TestAccountsSummaryDisclosuresAreCallouts(t *testing.T) {
	app := s4App(t,
		s4Account("bank", "Bank", "USD", domain.ClassAsset, domain.TypeChecking, 10000),
		s4Account("debt", "Debt", "EUR", domain.ClassLiability, domain.TypeOther, -1000))
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates = "USD", nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	f := s4Fixture(t)
	f.Render(ui.CreateElement(func() ui.Node {
		_ = uistate.UseDataRevision().Get()
		return ui.CreateElement(acctSummaryWidget, acctSummaryProps{
			App: app, Base: "USD", Rates: currency.Rates{Base: "USD", Rates: app.Settings().FXRates}})
	}))
	f.Stabilize()
	note := byTestID(f, "div", "acct-networth-note")
	if note == nil {
		t.Fatalf("missing the partial-total notice: %s", f.Text())
	}
	if class := note.Attr("class"); !strings.Contains(class, "callout") || !strings.Contains(class, "callout-warn") {
		t.Fatalf("partial-total notice is not a warn callout: class=%q", class)
	}
	if note.Attr("role") != "status" {
		t.Fatalf("partial-total notice role = %q, want status", note.Attr("role"))
	}
	for _, id := range []string{"acct-networth-value", "acct-assets-value", "acct-liabilities-value"} {
		if node := byTestID(f, "div", id); node == nil && byTestID(f, "span", id) == nil && byTestID(f, "a", id) == nil {
			t.Fatalf("headline lost %s: %s", id, f.Text())
		}
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "div", "acct-networth-note") != nil {
		t.Fatalf("the partial-total notice outlived its cause: %s", f.Text())
	}
}

// The toolbar keeps its search, the More menu trigger and, while balances are stale, the
// Mark all updated action. Typing in the search narrows the shared filter and shows its chip.
func TestAccountsToolbarSearchChipsAndActions(t *testing.T) {
	app := s4App(t,
		s4Account("chk", "Checking", "USD", domain.ClassAsset, domain.TypeChecking, 100),
		s4Account("sav", "Savings", "USD", domain.ClassAsset, domain.TypeSavings, 200))
	var filter state.Atom[uistate.AccountsFilter]
	f := s4Fixture(t)
	f.Render(ui.CreateElement(func() ui.Node {
		filter = uistate.UseAccountsFilter()
		return ui.CreateElement(acctToolbarWidget, acctToolbarProps{App: app})
	}))
	f.Stabilize()
	for _, tc := range []struct{ tag, id string }{
		{"input", "filter-toolbar-search"},
		{"button", "acct-manage-btn"},
		{"button", "acct-markall-btn"},
	} {
		if byTestID(f, tc.tag, tc.id) == nil {
			t.Fatalf("toolbar lost %s: %s", tc.id, f.Text())
		}
	}
	if byTestID(f, "div", "filter-summary") != nil {
		t.Fatal("an unfiltered toolbar showed filter chips")
	}
	filter.Set(uistate.AccountsFilter{Search: "sav"}.Normalize())
	f.Stabilize()
	summary := byTestID(f, "div", "filter-summary")
	if summary == nil || !strings.Contains(summary.Text(), "sav") {
		t.Fatalf("the search chip is missing: %s", f.Text())
	}
	search := byTestID(f, "input", "filter-toolbar-search")
	if search.Attr("role") != "searchbox" {
		t.Fatalf("search role = %q, want searchbox", search.Attr("role"))
	}
	search.Dispatch("onkeydown", render.Event{Key: "Escape"})
	f.Stabilize()
	if got := filter.Get().Search; got != "" {
		t.Fatalf("Escape did not clear the shared search filter: %q", got)
	}
}

// The account list header names the view and offers the class picker only when there is
// more than one class to pick. Choosing a class narrows the list and retitles it.
func TestAccountsListHeaderClassPicker(t *testing.T) {
	app := s4App(t,
		s4Account("chk", "Checking", "USD", domain.ClassAsset, domain.TypeChecking, 100),
		s4Account("card", "Card", "USD", domain.ClassLiability, domain.TypeCreditCard, -50))
	f := s4Fixture(t)
	f.Render(ui.CreateElement(acctListWidget, acctListProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}}))
	f.Stabilize()
	if !strings.Contains(f.Text(), uistate.T("accounts.allAccounts")) {
		t.Fatalf("list header lost its title: %s", f.Text())
	}
	for _, id := range []string{"acct-class-all", "acct-class-assets", "acct-class-liabilities", "acct-class-services"} {
		if byTestID(f, "button", id) == nil {
			t.Fatalf("class picker lost %s: %s", id, f.Text())
		}
	}
	byTestID(f, "button", "acct-class-liabilities").Click()
	f.Stabilize()
	if !strings.Contains(f.Text(), uistate.T("accounts.liabilitiesTitle")) {
		t.Fatalf("choosing Liabilities did not retitle the list: %s", f.Text())
	}
	if strings.Contains(f.Text(), "Checking") {
		t.Fatalf("choosing Liabilities left an asset in the list: %s", f.Text())
	}
}

// A search that matches nothing leaves a Callout where the rows were, not a bare paragraph.
func TestAccountsListNoMatchIsACallout(t *testing.T) {
	app := s4App(t, s4Account("chk", "Checking", "USD", domain.ClassAsset, domain.TypeChecking, 100))
	var filter state.Atom[uistate.AccountsFilter]
	f := s4Fixture(t)
	f.Render(ui.CreateElement(func() ui.Node {
		filter = uistate.UseAccountsFilter()
		return ui.CreateElement(acctListWidget, acctListProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}})
	}))
	f.Stabilize()
	if byTestID(f, "div", "acct-list-nomatch") != nil {
		t.Fatal("the no-match notice showed with no filter")
	}
	filter.Set(uistate.AccountsFilter{Search: "zzz-no-such-account"}.Normalize())
	f.Stabilize()
	notice := byTestID(f, "div", "acct-list-nomatch")
	if notice == nil {
		t.Fatalf("the no-match notice is missing: %s", f.Text())
	}
	if !strings.Contains(notice.Attr("class"), "callout") || !strings.Contains(notice.Text(), uistate.T("accounts.noMatch")) {
		t.Fatalf("no-match notice is not a callout with its copy: class=%q text=%q", notice.Attr("class"), notice.Text())
	}
}

// With most balances stale, one summary callout leads the list and carries the Mark all action.
func TestAccountsListStaleSummaryIsACalloutWithAction(t *testing.T) {
	app := s4App(t,
		s4Account("a", "Alpha", "USD", domain.ClassAsset, domain.TypeChecking, 100),
		s4Account("b", "Bravo", "USD", domain.ClassAsset, domain.TypeChecking, 100),
		s4Account("c", "Charlie", "USD", domain.ClassAsset, domain.TypeChecking, 100))
	f := s4Fixture(t)
	f.Render(ui.CreateElement(acctListWidget, acctListProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}}))
	f.Stabilize()
	summary := byTestID(f, "div", "acct-stale-summary")
	if summary == nil {
		t.Fatalf("the stale summary is missing: %s", f.Text())
	}
	if !strings.Contains(summary.Attr("class"), "callout") || summary.Attr("role") != "status" {
		t.Fatalf("stale summary is not a status callout: class=%q role=%q", summary.Attr("class"), summary.Attr("role"))
	}
	if byTestID(f, "button", "acct-stale-summary-markall") == nil {
		t.Fatalf("the stale summary lost its Mark all updated action: %s", summary.Text())
	}
}

func s4Buttons(f *render.Fixture) (submit *render.QueryNode) {
	for _, b := range f.AllByTag("button") {
		if b.Attr("type") == "submit" {
			return b
		}
	}
	return nil
}

// The transfer form keeps its test ids and gates submit on a valid pair and amount. Posting moves
// the money, closes the form, and a bad pair is refused.
func TestAccountsTransferFormFieldsGatingAndPost(t *testing.T) {
	app := s4App(t,
		s4Account("chk", "Checking", "USD", domain.ClassAsset, domain.TypeChecking, 100000),
		s4Account("sav", "Savings", "USD", domain.ClassAsset, domain.TypeSavings, 0))
	done := 0
	f := s4Fixture(t)
	f.Render(ui.CreateElement(AccountPageTransferForm, AccountPageTransferProps{App: app, OnDone: func() { done++ }}))
	f.Stabilize()
	form := byTestID(f, "form", "page-transfer-form")
	if form == nil || form.Attr("aria-label") != uistate.T("accounts.transferFormLabel") {
		t.Fatalf("transfer form lost its test id or label: %s", f.Text())
	}
	for _, tc := range []struct{ tag, id string }{
		{"select", "page-xfer-from-select"},
		{"select", "page-xfer-to-select"},
		{"input", "page-xfer-amt"},
	} {
		if byTestID(f, tc.tag, tc.id) == nil {
			t.Fatalf("transfer form lost %s: %s", tc.id, f.Text())
		}
	}
	if byTestID(f, "input", "page-xfer-received") != nil {
		t.Fatal("the received amount showed for a same-currency pair")
	}
	amount := byTestID(f, "input", "page-xfer-amt")
	if amount.Attr("type") != "number" || amount.Attr("id") != "page-xfer-amt" {
		t.Fatalf("amount input type=%q id=%q", amount.Attr("type"), amount.Attr("id"))
	}
	for _, label := range []string{"transferFromLabel", "transferToLabel", "transferAmount", "transferDateLabel", "transferDescLabel"} {
		if !strings.Contains(f.Text(), uistate.T("accounts."+label)) {
			t.Fatalf("transfer form lost the %s label: %s", label, f.Text())
		}
	}
	if submit := s4Buttons(f); submit == nil || !isDisabled(submit) {
		t.Fatal("submit was enabled before anything was filled in")
	}

	byTestID(f, "select", "page-xfer-from-select").Change("chk")
	byTestID(f, "select", "page-xfer-to-select").Change("sav")
	f.Stabilize()
	if byTestID(f, "input", "page-xfer-fee") == nil {
		t.Fatalf("the fee field is missing once a source account is picked: %s", f.Text())
	}
	if submit := s4Buttons(f); isDisabled(submit) != true {
		t.Fatal("submit was enabled with no amount")
	}
	byTestID(f, "input", "page-xfer-amt").Input("250.00")
	f.Stabilize()
	if !strings.Contains(f.Text(), uistate.T("accounts.xferPreviewTitle")) {
		t.Fatalf("the balance preview is missing once the transfer is valid: %s", f.Text())
	}
	if byTestID(f, "div", "xfer-balance-preview") == nil || byTestID(f, "span", "xfer-preview-from") == nil {
		t.Fatalf("the balance preview lost its test ids: %s", f.Text())
	}
	submit := s4Buttons(f)
	if isDisabled(submit) {
		t.Fatal("submit stayed disabled for a valid transfer")
	}
	byTestID(f, "form", "page-transfer-form").Submit()
	f.Stabilize()
	if done != 1 {
		t.Fatalf("OnDone calls = %d after a valid post, want 1", done)
	}
	if got := accountSnapshot(app).Balances["sav"].Amount; got != 25000 {
		t.Fatalf("savings balance after the transfer = %d, want 25000", got)
	}
}

// Picking the same account on both sides names the problem inside the destination field and
// keeps submit off. A cross-currency pair reveals the received amount.
func TestAccountsTransferFormErrorsAndCrossCurrency(t *testing.T) {
	app := s4App(t,
		s4Account("chk", "Checking", "USD", domain.ClassAsset, domain.TypeChecking, 100000),
		s4Account("eur", "Euro", "EUR", domain.ClassAsset, domain.TypeSavings, 0))
	f := s4Fixture(t)
	f.Render(ui.CreateElement(AccountPageTransferForm, AccountPageTransferProps{App: app}))
	f.Stabilize()
	byTestID(f, "select", "page-xfer-from-select").Change("chk")
	byTestID(f, "select", "page-xfer-to-select").Change("chk")
	f.Stabilize()
	var alert *render.QueryNode
	for _, n := range f.AllByTag("div") {
		if n.Attr("role") == "alert" && strings.Contains(n.Text(), uistate.T("accounts.transferSameAccountErr")) {
			alert = n
		}
	}
	if alert == nil {
		t.Fatalf("the same-account error is missing: %s", f.Text())
	}
	if !strings.Contains(alert.Attr("class"), "cf-field-error") {
		t.Fatalf("the same-account error is not the field's own error: class=%q", alert.Attr("class"))
	}
	byTestID(f, "input", "page-xfer-amt").Input("10")
	f.Stabilize()
	if !isDisabled(s4Buttons(f)) {
		t.Fatal("submit was enabled for a transfer to the same account")
	}
	byTestID(f, "select", "page-xfer-to-select").Change("eur")
	f.Stabilize()
	if byTestID(f, "input", "page-xfer-received") == nil {
		t.Fatalf("the received amount did not show for a cross-currency pair: %s", f.Text())
	}
}
