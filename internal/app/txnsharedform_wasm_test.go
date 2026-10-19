// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTxnAddAndEditSharePrimaryRowsAndLabels(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: "shared-form-account", Name: "Checking", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: "shared-form-savings", Name: "Savings", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "shared-form-category", Name: "Groceries", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	txn := domain.Transaction{ID: "shared-form-txn", AccountID: "shared-form-account", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Payee: "Market", Desc: "Market", Amount: money.New(-4200, "USD")}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	previousApp := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previousApp })
	func() {
		addFixture := render.New(t)
		defer addFixture.Cleanup()
		addFixture.Render(Body(ID("shared-form-body"), uic.CreateElement(QuickAddHost)))
		uistate.SetQuickAdd(true)
		addFixture.Render(Body(ID("shared-form-body"), uic.CreateElement(QuickAddHost)))
		assertTxnPrimaryRowsAndLabels(t, addFixture, []string{
			uistate.T("txu3.sentenceLabel"), uistate.T("quickAdd.amount"), uistate.T("quickAdd.kind"), uistate.T("quickAdd.expense"), uistate.T("quickAdd.income"), uistate.T("quickAdd.transfer"),
			uistate.T("quickAdd.payee"), uistate.T("quickAdd.date"), uistate.T("quickAdd.category"), uistate.T("quickAdd.account"),
		})
	}()
	t.Cleanup(func() { uistate.SetQuickAdd(false) })

	editFixture := render.New(t)
	defer editFixture.Cleanup()
	editFixture.Render(uic.CreateElement(screens.TransactionEditForm, screens.TransactionEditFormProps{TxnID: txn.ID}))
	assertTxnPrimaryRowsAndLabels(t, editFixture, []string{
		uistate.T("quickAdd.amount"), uistate.T("quickAdd.kind"), uistate.T("quickAdd.expense"), uistate.T("quickAdd.income"), uistate.T("quickAdd.transfer"),
		uistate.T("quickAdd.payee"), uistate.T("quickAdd.date"), uistate.T("quickAdd.category"), uistate.T("quickAdd.account"),
	})
}

func assertTxnPrimaryRowsAndLabels(t *testing.T, fixture *render.Fixture, labels []string) {
	t.Helper()
	var grid *render.QueryNode
	for _, node := range fixture.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "txn-form-grid") {
			grid = node
			break
		}
	}
	if grid == nil {
		t.Fatal("shared transaction field grid is missing")
	}
	text := fixture.Text()
	previous := -1
	for _, label := range labels {
		position := strings.Index(text, label)
		if position <= previous {
			t.Fatalf("shared form text %q does not place %q in the required order", text, label)
		}
		previous = position
	}
}
