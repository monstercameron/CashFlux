// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTxnBulkCategoryPickerOffersKeyboardCreateLast(t *testing.T) {
	var created []string
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnBulkPicker, txnBulkPickerProps{
		ID: "bulk-category-picker", Label: "Category", Placeholder: "Category", SearchLabel: "Search categories",
		Options:  []txnBulkPickerOption{{Value: "cat-food", Label: "Food"}},
		OnChoose: func(string) {}, OnCreate: func(name string) { created = append(created, name) },
	}))
	f.ByID("bulk-category-picker").Click()
	search := f.ByID("bulk-category-picker-search")
	search.Input("Fresh category")
	f.Stabilize()
	create := f.ByID("bulk-category-picker-option-0")
	if create == nil || create.Attr("role") != "option" {
		t.Fatal("unmatched category did not expose a keyboard-reachable create option")
	}
	if got := create.Text(); got != "Create ‘Fresh category’" {
		t.Fatalf("create option label = %q", got)
	}
	search.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if len(created) != 1 || created[0] != "Fresh category" {
		t.Fatalf("Enter create callback = %v, want exactly [Fresh category]", created)
	}
}

func TestTxnBulkCreateOpensPrefilledPickerAndPicksCreatedCategoryOnce(t *testing.T) {
	app := newR3EEditApp(t)

	picked := make([]string, 0, 1)
	f := render.New(t)
	pickerFixtureTarget(t, f)
	defer f.Cleanup()
	f.Render(txnBulkCategoryPicker("Fresh category", domain.KindExpense,
		func(id string) { picked = append(picked, id) }, func() {}))
	search := findByTestID(f, "input", "catpick-search")
	create := findByTestID(f, "button", "catpick-create")
	if search == nil || !strings.Contains(f.Text(), "Fresh category") || create == nil {
		t.Fatalf("picker did not render its prefilled category name and create action: %q", f.Text())
	}
	create.Click()
	if len(picked) != 1 {
		t.Fatalf("picker selected %v, want exactly one created category", picked)
	}
	found := false
	for _, category := range app.Categories() {
		if category.ID == picked[0] && category.Name == "Fresh category" && category.Kind == domain.KindExpense {
			found = true
		}
	}
	if !found {
		t.Fatalf("picked id %q does not identify the created expense category", picked[0])
	}
}
