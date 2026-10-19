// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestEditCategoryPickerCreatesAndSelectsSubcategoryCAT1(t *testing.T) {
	app := newR3EEditApp(t)
	if err := app.PutCategory(domain.Category{ID: "cat-home", Name: "Home", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "cat-utilities", Name: "Utilities", Kind: domain.KindExpense, ParentID: "cat-home"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "cat-income", Name: "Pay", Kind: domain.KindIncome}); err != nil {
		t.Fatal(err)
	}
	txn := domain.Transaction{ID: "cat1-edit", AccountID: "acct-1", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Desc: "Home repair", Amount: money.New(-1000, "USD")}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}

	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{TxnID: txn.ID}))
	selectNode := findByTestID(f, "select", "txn-edit-category")
	if selectNode == nil || !strings.Contains(selectNode.Text(), "Home > Utilities") || strings.Contains(selectNode.Text(), "Pay") {
		t.Fatalf("expense options should be kind-filtered and full-path: %#v / %q", selectNode, selectNode.Text())
	}
	findByTestID(f, "button", "txn-edit-direction-income").Click()
	f.Stabilize()
	selectNode = findByTestID(f, "select", "txn-edit-category")
	if selectNode == nil || !strings.Contains(selectNode.Text(), "Pay") || strings.Contains(selectNode.Text(), "Groceries") || strings.Contains(selectNode.Text(), "Utilities") {
		t.Fatalf("income options should refresh after switching direction: %q", selectNode.Text())
	}
	selectNode.Change(txnEditNewCategoryOption)
	f.Stabilize()
	if findByTestID(f, "div", "catpick") == nil {
		t.Fatal("New category option did not open CategoryPicker")
	}
	findByTestID(f, "input", "catpick-search").Input("Side work")
	f.Stabilize()
	findByTestID(f, "select", "catpick-parent").Change("cat-income")
	f.Stabilize()
	findByTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	var created domain.Category
	for _, category := range app.Categories() {
		if category.Name == "Side work" {
			created = category
		}
	}
	if created.ID == "" || created.ParentID != "cat-income" || created.Kind != domain.KindIncome {
		t.Fatalf("created category = %#v, want an income child of cat-income", created)
	}
	selected := false
	for _, option := range f.AllByTag("option") {
		if option.Text() == "Pay > Side work" && option.Property("selected") == true {
			selected = true
		}
	}
	if !selected {
		t.Fatalf("new category %q is not selected in Edit", created.ID)
	}
}
