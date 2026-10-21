// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func txb13App(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	for _, c := range []domain.Category{
		{ID: "cat-parent", Name: "Shopping", Kind: domain.KindExpense},
		{ID: "cat-grocery", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "cat-household", Name: "Household", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(c); err != nil {
			t.Fatalf("PutCategory(%q): %v", c.ID, err)
		}
	}
	if err := app.PutAccount(domain.Account{
		ID: "acct", Name: "Checking", Currency: "USD", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	return app
}

func TestTransactionSplitFormReseedsAcceptedProposal(t *testing.T) {
	app := txb13App(t)
	prev := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = prev })
	txn := domain.Transaction{
		ID: "txn-proposal", AccountID: "acct", Desc: "Receipt", CategoryID: "cat-parent",
		Amount: money.New(-800, "USD"), Date: time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC),
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	uistate.SetTxnSplitProposal(txn.ID, []domain.CategorySplit{
		{CategoryID: "cat-grocery", Amount: money.New(-500, "USD")},
		{CategoryID: "cat-household", Amount: money.New(-300, "USD")},
	}, "review these lines")

	f := render.New(t)
	f.Render(ui.CreateElement(TransactionSplitForm, TransactionSplitFormProps{TxnID: txn.ID}))
	f.Stabilize()

	if save := byTestID(f, "button", "split-save"); save == nil || isDisabled(save) {
		t.Fatal("accepted balanced proposal did not reseed the editor into a saveable two-line draft")
	}
}

func TestProposeFromRowsKeepsCategoriesForDuplicateDescriptions(t *testing.T) {
	app := txb13App(t)
	txn := domain.Transaction{ID: "txn-receipt", CategoryID: "cat-parent", Amount: money.New(-800, "USD")}
	proposeFromRows(app, txn, []extract.Row{
		{Description: "Item", Amount: "$5.00", Category: "Groceries"},
		{Description: " Item ", Amount: "$3.00", Category: "Household"},
	})

	splits, _, ok := uistate.TakeTxnSplitProposal(txn.ID)
	if !ok {
		t.Fatal("duplicate-description receipt rows did not produce a proposal")
	}
	got := map[string]int64{}
	for _, line := range splits {
		got[line.CategoryID] += line.Amount.Amount
	}
	if got["cat-grocery"] != -500 || got["cat-household"] != -300 {
		t.Fatalf("duplicate descriptions did not retain per-line categories: %+v", got)
	}
}
