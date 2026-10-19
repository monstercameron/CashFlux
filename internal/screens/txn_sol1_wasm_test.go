// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSOL1EditorFailureDoesNotExposeInternalErrorText(t *testing.T) {
	got := txnEditSaveError(errors.New("database failure with private implementation details"))
	if got != uistate.T("sol1.saveFailed") || strings.Contains(got, "database") {
		t.Fatalf("editor exposed internal failure: %q", got)
	}
}

func TestSOL1OnlyDescriptionAndPencilOpenEdit(t *testing.T) {
	opened := []string{}
	f := render.New(t)
	defer f.Cleanup()
	props := &txnFrameRowProps{
		ID: "sol1-row", Desc: "Corner store", Date: "Oct 2", Amount: "$6.75",
		AmountMoney: money.New(-675, "USD"),
		OnOpen:      func(id string) { opened = append(opened, id) },
	}
	f.Render(Table(Tbody(ui.CreateElement(txnFrameRow, props))))
	row := findByTestID(f, "tr", "txn-row-sol1-row")
	if row == nil || row.Property("onclick") != nil {
		t.Fatal("ledger row must have no click handler")
	}
	for _, cell := range row.Children() {
		if cell.Property("onclick") != nil {
			cell.Click()
		}
	}
	if len(opened) != 0 {
		t.Fatalf("clicking row/cells opened Edit: %v", opened)
	}
	description := findByTestID(f, "button", "txn-description-edit")
	name := txnRowActionName(uistate.T("transactions.rowEdit"), props.Desc, props.Date, props.Amount)
	if description == nil || description.Name() != name || description.Attr("type") != "button" || description.Attr("tabindex") == "-1" {
		t.Fatalf("description is not a keyboard reachable named edit button: %#v", description)
	}
	description.Click()
	findByTestID(f, "button", "txn-rowedit").Click()
	if len(opened) != 2 || opened[0] != props.ID || opened[1] != props.ID {
		t.Fatalf("description/pencil actions = %v, want the same record twice", opened)
	}
}

func TestSOL1EditRecordSummaryAndAssociatedValidation(t *testing.T) {
	app := sol1EditApp(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(TransactionEditForm(TransactionEditFormProps{App: app, TxnID: "sol1-record"}))
	record := findByTestID(f, "div", "txn-edit-record")
	if record == nil || !strings.Contains(record.Text(), "CORNER STORE 123") || !strings.Contains(record.Text(), "Corner store") || !strings.Contains(record.Text(), "2026") {
		t.Fatalf("record summary does not identify description, merchant and date: %q", record.Text())
	}
	if amount := f.ByID("txn-edit-amount"); amount == nil || amount.Attr("data-cf-value") != "6.75" {
		t.Fatalf("editable amount must retain the transaction amount: %v", amount)
	}
	for _, tc := range []struct{ id, label string }{
		{"txn-edit-amount", uistate.T("quickAdd.amount")},
		{"txn-edit-payee", uistate.T("quickAdd.payee")},
		{"txn-edit-date", uistate.T("quickAdd.date")},
		{"txn-edit-description", uistate.T("transactions.descPlaceholder")},
	} {
		control := f.ByID(tc.id)
		if control == nil || control.Name() != tc.label {
			t.Fatalf("control %s has no associated accessible label %q", tc.id, tc.label)
		}
	}
	typeGroup := f.ByRole("group", uistate.T("quickAdd.kind"))
	if typeGroup == nil {
		t.Fatal("transaction type choices are not a named group")
	}
	transfer := findByTestID(f, "button", "txn-edit-direction-transfer")
	if transfer == nil || transfer.Attr("disabled") != "" || transfer.Attr("title") != uistate.T("r9f.transferEditUnavailable") {
		t.Fatal("transfer choice should be enabled and explain the limit on hover or focus")
	}
	if f.ByID("txn-edit-more-sol1-record").Attr("open") != "" {
		t.Fatal("optional details must start collapsed")
	}
	f.ByID("txn-edit-amount").Input("not money")
	f.ByID("txn-edit-form").Submit()
	amount := f.ByID("txn-edit-amount")
	if amount.Attr("aria-invalid") != "true" || !strings.Contains(amount.Attr("aria-describedby"), "txn-edit-amount-error") || f.ByID("txn-edit-amount-error") == nil {
		t.Fatal("amount failure is not associated with the field")
	}
	if got := app.Transactions()[0].Amount.Amount; got != -675 {
		t.Fatalf("invalid submission changed the transaction to %d", got)
	}
	f.ByID("txn-edit-amount").Input("6.75")
	f.ByID("txn-edit-date").Input("")
	f.ByID("txn-edit-form").Submit()
	if f.ByID("txn-edit-date").Attr("aria-invalid") != "true" || f.ByID("txn-edit-date-error") == nil {
		t.Fatal("date failure is not associated with the field")
	}
}

func sol1EditApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	if err := app.PutAccount(domain.Account{ID: "sol1-account", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "sol1-record", AccountID: "sol1-account", Payee: "Corner store", Desc: "CORNER STORE 123", Date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Amount: money.New(-675, "USD")}); err != nil {
		t.Fatal(err)
	}
	return app
}
