// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR10WLedgerCategoryPickerCreatesAndApplies(t *testing.T) {
	app := newR3EEditApp(t)
	txn := domain.Transaction{
		ID: "r10w-transaction", AccountID: "acct-1", Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC),
		Desc: "Market purchase", Amount: money.New(-4200, "USD"),
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	props := r4cRow(txn.ID)
	props.Category = "Uncategorized"
	props.CategoryID = ""
	props.Unfiled = true
	f := render.New(t)
	defer f.Cleanup()
	props.CategoryPickerTarget = ui.PortalTarget{Node: f.Target()}
	f.Render(ui.CreateElement(txnFrameRow, &props))

	choose := findByTestID(f, "button", "txn-category-choose")
	if choose == nil || choose.Text() != "Uncategorized" {
		t.Fatalf("category cell chooser = %v, want the Uncategorized affordance", choose)
	}
	choose.Click()
	f.Stabilize()
	if findByTestID(f, "div", "catpick") == nil {
		t.Fatal("ledger category cell did not open the shared CategoryPicker")
	}
	findByTestID(f, "input", "catpick-search").Input("Ledger category")
	f.Stabilize()
	findByTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	var created domain.Category
	for _, category := range app.Categories() {
		if category.Name == "Ledger category" {
			created = category
		}
	}
	if created.ID == "" || created.Kind != domain.KindExpense {
		t.Fatalf("created category = %#v, want a new expense category", created)
	}
	updated, ok := txnByID(app, txn.ID)
	if !ok || updated.CategoryID != created.ID || !updated.Reviewed {
		t.Fatalf("ledger transaction after category choice = %#v; want category %q and Reviewed", updated, created.ID)
	}
	if findByTestID(f, "div", "catpick") != nil {
		t.Fatal("CategoryPicker stayed open after the new category was selected")
	}
}

func TestR10WSuggestionLabelHasNoLeadingSeparator(t *testing.T) {
	if got := uistate.T("r10w.suggestionInline", "Dining"); got != "Dining?" {
		t.Fatalf("inline suggestion label = %q, want Dining? with no separator", got)
	}
}

func TestR10WOverflowSubmenuKeyboardOpenAndClose(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(uiw.OverflowMenu(uiw.OverflowMenuProps{TriggerTestID: "r10w-trigger", Items: []uiw.OverflowMenuItem{{
		Label: "Link to…", TestID: "r10w-link", Submenu: []uiw.OverflowMenuItem{{Label: "Mark as bill payment", TestID: "r10w-bill"}},
	}}}))
	findByTestID(f, "button", "r10w-trigger").Click()
	link := findByTestID(f, "button", "r10w-link")
	link.Dispatch("onkeydown", render.Event{Key: "ArrowRight"})
	if findByTestID(f, "button", "r10w-bill") == nil {
		t.Fatal("Right did not open the submenu")
	}
	findByTestID(f, "button", "r10w-bill").Dispatch("onkeydown", render.Event{Key: "ArrowLeft"})
	if findByTestID(f, "button", "r10w-bill") != nil {
		t.Fatal("Left did not close the submenu")
	}
	link.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if findByTestID(f, "button", "r10w-bill") == nil {
		t.Fatal("Enter did not open the submenu")
	}
	findByTestID(f, "button", "r10w-bill").Dispatch("onkeydown", render.Event{Key: "Escape"})
	if findByTestID(f, "button", "r10w-bill") != nil {
		t.Fatal("Escape did not close the submenu")
	}
}
