// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR3EEditFormUsesSharedOrderAndKeepsBodyActions(t *testing.T) {
	app := newR3EEditApp(t)
	txn := domain.Transaction{
		ID: "r3e-expense", AccountID: "acct-1", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Payee: "Market", Desc: "MARKET #123 ORIGINAL BANK TEXT", Amount: money.New(-4200, "USD"),
		Splits:      []domain.CategorySplit{{CategoryID: "cat-food", Amount: money.New(-4200, "USD")}},
		Attachments: []domain.AttachmentRef{{ArtifactID: "receipt-1", Name: "receipt.jpg"}},
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	fixture := render.New(t)
	defer fixture.Cleanup()
	fixture.Render(ui.Fragment(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{TxnID: txn.ID}),
		ui.CreateElement(TxnEditDeleteFooter, TransactionEditFormProps{TxnID: txn.ID})))
	text := fixture.Text()
	ordered := []string{
		uistate.T("transactions.amountPlaceholder"), uistate.T("quickAdd.expense"),
		uistate.T("transactions.payeeLabel"), uistate.T("transactions.categoryLabel"),
		uistate.T("transactions.dateLabel"), uistate.T("quickAdd.account"),
		uistate.T("r1e.moreDetails"), uistate.T("transactions.descPlaceholder"),
		uistate.T("transactions.tagsLabel"), uistate.T("transactions.noteLabel"),
		uistate.T("txnwidget.clearedLabel"), uistate.T("transactions.excludeLabel"),
	}
	last := -1
	for _, label := range ordered {
		at := strings.Index(text, label)
		if at <= last {
			t.Fatalf("field %q is out of shared-form order in %q", label, text)
		}
		last = at
	}
	if findByTestID(fixture, "p", "txn-edit-bank-text") != nil || strings.Contains(text, "Bank text:") {
		t.Fatalf("bank text must be absent without an original-descriptor field: %q", text)
	}
	for _, check := range []struct{ tag, id string }{
		{"select", "txn-edit-account"}, {"select", "txn-edit-category"}, {"button", "txn-edit-direction-expense"}, {"button", "txn-edit-direction-income"},
		{"div", "txn-edit-splits"}, {"button", "txn-edit-newcat-toggle"}, {"button", "txn-edit-delete"},
	} {
		if findByTestID(fixture, check.tag, check.id) == nil {
			t.Errorf("edit form is missing %s", check.id)
		}
	}
	deleteButton := findByTestID(fixture, "button", "txn-edit-delete")
	if deleteButton == nil || !strings.Contains(deleteButton.Attr("class"), "txn-edit-delete") {
		t.Fatalf("Delete should be a visibly destructive action: %#v", deleteButton)
	}
	if fixture.ByID("txn-edit-more-r3e-expense").Attr("open") != "" {
		t.Fatal("More details should start collapsed")
	}
	expense := findByTestID(fixture, "button", "txn-edit-direction-expense")
	income := findByTestID(fixture, "button", "txn-edit-direction-income")
	if expense == nil || !strings.Contains(expense.Attr("class"), "is-pressed") || expense.Attr("aria-pressed") != "true" {
		t.Fatalf("expense direction lacks visible pressed state: %#v", expense)
	}
	if income == nil || strings.Contains(income.Attr("class"), "is-pressed") || income.Attr("aria-pressed") != "false" {
		t.Fatalf("income direction is unexpectedly pressed: %#v", income)
	}
	if findByTestID(fixture, "button", "txn-edit-more-actions") == nil {
		t.Fatal("Tools menu is missing")
	} else if findByTestID(fixture, "button", "txn-edit-more-actions").Attr("aria-label") != uistate.T("r9f.tools") {
		t.Fatal("Edit menu should be named Tools")
	}
	transferChoice := findByTestID(fixture, "button", "txn-edit-direction-transfer")
	if transferChoice == nil || transferChoice.Attr("disabled") == "" || transferChoice.Attr("title") == "" {
		t.Fatalf("Edit transfer conversion must remain visible with an explanation: %#v", transferChoice)
	}
	newCategory := findByTestID(fixture, "button", "txn-edit-newcat-toggle")
	if newCategory.Attr("aria-label") != uistate.T("transactions.newCategory") || newCategory.Attr("title") != uistate.T("transactions.newCategory") {
		t.Fatalf("New category control lacks its accessible name and tooltip: %#v", newCategory)
	}
	findByTestID(fixture, "button", "txn-edit-more-actions").Click()
	for _, id := range []string{"txn-edit-split-toggle", "txn-edit-attach"} {
		if findByTestID(fixture, "button", id) == nil {
			t.Errorf("More actions menu cannot reach %s", id)
		}
	}
}

func TestR3ESuggestionIsOfferedOnlyForMatchingDirection(t *testing.T) {
	app := newR3EEditApp(t)
	if err := app.PutRule(rules.Rule{ID: "r3e-rule", Match: "SAMPLE MERCHANT", SetCategoryID: "cat-food"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, id string
		amount   int64
		want     bool
	}{
		{name: "uncategorized expense", id: "r3e-expense-suggest", amount: -1000, want: true},
		{name: "uncategorized income rejects expense category", id: "r3e-income-suggest", amount: 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			txn := domain.Transaction{ID: tc.id, AccountID: "acct-1", Date: time.Now(), Payee: "SAMPLE MERCHANT", Desc: "SAMPLE MERCHANT", Amount: money.New(tc.amount, "USD")}
			if err := app.PutTransaction(txn); err != nil {
				t.Fatal(err)
			}
			fixture := render.New(t)
			defer fixture.Cleanup()
			fixture.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{TxnID: txn.ID}))
			got := findByTestID(fixture, "button", "txn-edit-category-suggestion") != nil
			if got != tc.want {
				t.Fatalf("suggestion visible = %v, want %v; form text: %q", got, tc.want, fixture.Text())
			}
			if got && !strings.Contains(fixture.Text(), "Suggested: Groceries · Use") {
				t.Fatalf("suggestion is not a clear one-control action: %q", fixture.Text())
			}
		})
	}
}

func newR3EEditApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []domain.Account{
		{ID: "acct-1", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "acct-2", Name: "Savings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD"},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutCategory(domain.Category{ID: "cat-food", Name: "Groceries", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })
	return app
}
