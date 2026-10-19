// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBulkSetOwnersSnapshotCanBeUndone(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	original := domain.Transaction{ID: "bulk-shared", AccountID: "account", Desc: "Shared purchase", Amount: money.New(-1000, "USD"), Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), MemberIDs: []string{"m1", "m2"}}
	if err := app.PutTransaction(original); err != nil {
		t.Fatal(err)
	}

	prior, err := assignSelectedTxnOwners(app, map[string]bool{original.ID: true}, []string{"m3", "m4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prior) != 1 || !reflect.DeepEqual(prior[0].Owners(), original.Owners()) {
		t.Fatalf("undo snapshot = %#v, want original owner list %#v", prior, original.Owners())
	}
	updated, ok := findTransactionByID(app.Transactions(), original.ID)
	if !ok || !reflect.DeepEqual(updated.Owners(), []string{"m3", "m4"}) {
		t.Fatalf("bulk owner replacement = %#v, exists=%t", updated.Owners(), ok)
	}
	if err := app.RestoreTransactions(prior); err != nil {
		t.Fatal(err)
	}
	undone, ok := findTransactionByID(app.Transactions(), original.ID)
	if !ok || !reflect.DeepEqual(undone.Owners(), original.Owners()) {
		t.Fatalf("undo restored owners = %#v, exists=%t; want %#v", undone.Owners(), ok, original.Owners())
	}
}

func TestBulkUnassignedPickerChoiceShowsUndoAndRestoresOwners(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	original := domain.Transaction{ID: "bulk-ui", AccountID: "account", Desc: "Shared purchase", Amount: money.New(-1000, "USD"), Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), MemberIDs: []string{"m1", "m2"}}
	if err := app.PutTransaction(original); err != nil {
		t.Fatal(err)
	}
	selected := toggleTxnBulkOwner(nil, txnBulkUnassignedOwner)
	if len(selected) != 1 || selected[0] != txnBulkUnassignedOwner {
		t.Fatalf("selected Unassigned option = %#v", selected)
	}
	owners := txnBulkAppliedOwners(selected)
	if len(owners) != 0 {
		t.Fatalf("committed Unassigned choice = %#v, want no owners", owners)
	}
	prior, err := assignSelectedTxnOwners(app, map[string]bool{original.ID: true}, owners)
	if err != nil {
		t.Fatalf("apply Unassigned choice: %v", err)
	}
	updated, ok := findTransactionByID(app.Transactions(), original.ID)
	if !ok || len(updated.Owners()) != 0 {
		t.Fatalf("bulk unassigned action owners = %#v, exists=%t", updated.Owners(), ok)
	}
	if err := app.RestoreTransactions(prior); err != nil {
		t.Fatalf("restore owner snapshot: %v", err)
	}
	undone, ok := findTransactionByID(app.Transactions(), original.ID)
	if !ok || !reflect.DeepEqual(undone.Owners(), original.Owners()) {
		t.Fatalf("undo owners = %#v, exists=%t; want %#v", undone.Owners(), ok, original.Owners())
	}
}

func txnBulkBarSelectionHarness(app *appstate.App) ui.Node {
	seeded := ui.UseState(false)
	selection := uistate.UseTxnSelection()
	if !seeded.Get() {
		selection.Set(map[string]bool{"bulk-ui": true})
		seeded.Set(true)
	}
	return ui.CreateElement(txnBulkBarWidget, txnBulkBarProps{App: app})
}

func findTransactionByID(transactions []domain.Transaction, id string) (domain.Transaction, bool) {
	for _, txn := range transactions {
		if txn.ID == id {
			return txn, true
		}
	}
	return domain.Transaction{}, false
}
