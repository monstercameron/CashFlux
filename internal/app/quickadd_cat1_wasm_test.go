// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestQuickAddCategoryPickerCreatesAndSelectsSubcategoryCAT1(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []domain.Category{
		{ID: "cat-add-home", Name: "Home", Kind: domain.KindExpense},
		{ID: "cat-add-income", Name: "Pay", Kind: domain.KindIncome},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous; uistate.SetQuickAdd(false) })

	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("cat1-body"), ui.CreateElement(QuickAddHost)))
	uistate.SetQuickAdd(true)
	f.Render(Body(ID("cat1-body"), ui.CreateElement(QuickAddHost)))
	findAppTestID(f, "button", "txn-add-newcat-toggle").Click()
	f.Stabilize()
	if findAppTestID(f, "div", "catpick") == nil {
		t.Fatal("Add + did not open CategoryPicker")
	}
	findAppTestID(f, "input", "catpick-search").Input("Repairs")
	f.Stabilize()
	findAppTestID(f, "select", "catpick-parent").Change("cat-add-home")
	f.Stabilize()
	findAppTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	var created domain.Category
	for _, category := range app.Categories() {
		if category.Name == "Repairs" {
			created = category
		}
	}
	if created.ID == "" || created.ParentID != "cat-add-home" || created.Kind != domain.KindExpense {
		t.Fatalf("created category = %#v, want an expense child of cat-add-home", created)
	}
	var selected bool
	for _, option := range f.AllByTag("option") {
		if option.Text() == "Home > Repairs" && option.Property("selected") == true {
			selected = true
		}
	}
	if !selected {
		t.Fatalf("new category %q is not selected in Add", created.ID)
	}
}

func findAppTestID(f *render.Fixture, tag, id string) *render.QueryNode {
	for _, node := range f.AllByTag(tag) {
		if node.Attr("data-testid") == id {
			return node
		}
	}
	return nil
}
