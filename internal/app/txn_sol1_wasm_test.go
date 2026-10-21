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
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestSOL1EditOverlayFooterAndModalContract(t *testing.T) {
	app := sol1FormApp(t)
	var durableRevision uint64
	var completeDurable func(appstate.DurableResult)
	app.SetDurableWriter(func(revision uint64, done func(appstate.DurableResult)) {
		durableRevision = revision
		completeDurable = done
	})
	closed := 0
	close := func() { closed++ }
	formProps := screens.TransactionEditFormProps{App: app, TxnID: "sol1-txn", OnDone: close}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("sol1-body"), uic.CreateElement(txnEditOverlay, txnEditOverlayProps{
		Open: true,
		Body: screens.TransactionEditForm(formProps), Delete: screens.TxnEditDeleteFooter(formProps), OnClose: close,
	})))
	dialog := f.ByRole("dialog", uistate.T("transactions.editTitle"))
	if dialog == nil || dialog.Attr("aria-modal") != "true" || f.BuildOverlayFocusSurfaceID() == "" || f.BuildOverlayEscapeSurfaceID() == "" || !f.BuildOverlayScrollLockActive() {
		t.Fatal("Edit does not own the shared modal contract")
	}
	if f.BuildOverlaySurfaces()[0].PortalTargetID != "sol1-body" {
		t.Fatal("Edit is not portalled to the body")
	}
	for _, id := range []string{"txn-edit-delete", "txn-edit-cancel", "txn-edit-save"} {
		if findAppTestID(f, "button", id) == nil {
			t.Fatalf("missing pinned footer action %s", id)
		}
	}
	save := findAppTestID(f, "button", "txn-edit-save")
	if save.Attr("type") != "submit" || save.Attr("form") != "txn-edit-form" {
		t.Fatal("Save must natively submit the edit form")
	}
	findAppTestID(f, "button", "txn-edit-cancel").Click()
	if closed != 1 {
		t.Fatalf("Cancel emitted %d close actions", closed)
	}
	if !f.HandleOverlayOutsideClick(f.BuildOverlaySurfaces()[0].SurfaceID) || closed != 2 {
		t.Fatal("shared backdrop dismissal did not emit close")
	}
	f.ByID("txn-edit-amount").Input("8.50")
	f.ByID("txn-edit-form").Submit()
	if closed != 2 || completeDurable == nil {
		t.Fatal("edit closed before its durable write completed")
	}
	completeDurable(appstate.DurableResult{Revision: durableRevision})
	if closed != 3 || app.Transactions()[0].Amount.Amount != -850 {
		t.Fatal("native form submission did not save and close")
	}
}

func TestSOL1AddAndEditExposeTheSamePrimaryFieldStructure(t *testing.T) {
	for _, mode := range []string{"add", "edit"} {
		t.Run(mode, func(t *testing.T) {
			app := sol1FormApp(t)
			f := render.New(t)
			defer f.Cleanup()
			var content uic.Node
			if mode == "add" {
				content = uic.CreateElement(quickAddForm, quickAddFormProps{App: app, Open: true})
			} else {
				content = screens.TransactionEditForm(screens.TransactionEditFormProps{App: app, TxnID: "sol1-txn"})
			}
			f.Render(Body(ID("sol1-structure-body"), content))
			var grid *render.QueryNode
			for _, node := range f.AllByTag("div") {
				if strings.Contains(node.Attr("class"), "txn-form-grid") {
					grid = node
					break
				}
			}
			if grid == nil {
				t.Fatal("missing shared two-column transaction grid")
			}
			for _, label := range []string{uistate.T("quickAdd.amount"), uistate.T("quickAdd.payee"), uistate.T("quickAdd.date")} {
				if f.ByRole("textbox", label) == nil && label != uistate.T("quickAdd.date") {
					t.Fatalf("missing named primary control %q", label)
				}
			}
			if f.ByRole("group", uistate.T("quickAdd.kind")) == nil {
				t.Fatal("Type is not a named control group")
			}
			if mode == "add" {
				if f.ByRole("dialog", uistate.T("quickAdd.title")) == nil || f.BuildOverlayFocusSurfaceID() == "" {
					t.Fatal("Add must share Edit's Overlay presentation")
				}
				for _, id := range []string{"txn-add-cancel", "txn-add-another", "flip-save"} {
					if findAppTestID(f, "button", id) == nil {
						t.Fatalf("missing Add footer action %s", id)
					}
				}
			}
		})
	}
}

func TestSOL1AddFieldCallbacksAndPinnedSaveKeepTheirBehavior(t *testing.T) {
	app := sol1FormApp(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("sol1-add-save-body"), uic.CreateElement(quickAddForm, quickAddFormProps{App: app, Open: true})))
	f.ByID("txn-add-amount").Input("12.34")
	f.ByID("txn-add-payee").Input("Bakery")
	findAppTestID(f, "button", "flip-save").Click()
	for _, txn := range app.Transactions() {
		if txn.ID != "sol1-txn" {
			if txn.Payee != "Bakery" || txn.Desc != "Bakery" || txn.Amount.Amount != -1234 {
				t.Fatalf("Add fields saved the wrong values: %#v", txn)
			}
			return
		}
	}
	t.Fatal("pinned Save did not create the transaction")
}

func TestSOL1HostsHonorControlledOpenAcrossCloseAndReopen(t *testing.T) {
	for _, mode := range []string{"add", "edit"} {
		t.Run(mode, func(t *testing.T) {
			app := sol1FormApp(t)
			f := render.New(t)
			defer f.Cleanup()
			for _, open := range []bool{false, true, false, true, false} {
				var content uic.Node
				if mode == "add" {
					content = uic.CreateElement(quickAddForm, quickAddFormProps{App: app, Open: open})
				} else {
					content = uic.CreateElement(txnEditOverlay, txnEditOverlayProps{Open: open, Body: Input(Type("text"))})
				}
				f.Render(Body(ID("sol1-controlled-body"), content))
				if got := len(f.BuildOverlaySurfaces()) > 0; got != open {
					t.Fatalf("Open=%t rendered surface=%t", open, got)
				}
				if f.BuildOverlayScrollLockActive() != open {
					t.Fatalf("Open=%t has incorrect scroll lock", open)
				}
			}
		})
	}
}

func sol1FormApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	for _, account := range []domain.Account{
		{ID: "sol1-checking", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "sol1-savings", Name: "Savings", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeSavings, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutTransaction(domain.Transaction{ID: "sol1-txn", AccountID: "sol1-checking", Payee: "Corner store", Desc: "CORNER STORE", Date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Amount: money.New(-675, "USD")}); err != nil {
		t.Fatal(err)
	}
	return app
}
