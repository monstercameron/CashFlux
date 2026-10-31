// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestV5FORMAccountPickerDisambiguatesDuplicateNamesAndKeepsPickerOrder(t *testing.T) {
	accounts := []domain.Account{
		{ID: "checking", Name: "Everyday", Type: domain.TypeChecking},
		{ID: "card", Name: "Everyday", Type: domain.TypeCreditCard},
		{ID: "savings", Name: "Reserve", Type: domain.TypeSavings},
	}
	options := TxnFormAccountOptions(accounts, "")
	if len(options) != len(accounts) {
		t.Fatalf("got %d account options, want %d", len(options), len(accounts))
	}
	if options[0].Value != "card" || options[0].Label != "Everyday · Credit card" {
		t.Errorf("first picker option = %#v, want credit card first with a type cue", options[0])
	}
	if options[1].Value != "checking" || options[1].Label != "Everyday · Checking" {
		t.Errorf("second picker option = %#v, want checking second with a type cue", options[1])
	}
	if options[2].Value != "savings" || options[2].Label != "Reserve" {
		t.Errorf("unique-name option = %#v, want its concise account name", options[2])
	}
}

func TestV5FORMAccountPickerCanRetainArchivedOriginalDuringMove(t *testing.T) {
	options := TxnFormAccountOptions([]domain.Account{
		{ID: "old", Name: "Old bank", Type: domain.TypeChecking, Archived: true},
		{ID: "new", Name: "New bank", Type: domain.TypeChecking},
	}, "new", "old")
	if len(options) != 2 || options[0].Value != "new" || options[1].Value != "old" {
		t.Fatalf("options after selecting a new account = %#v, want new and retained original archived account", options)
	}
}

func TestSplitNewCategoryDismissalRestoresSelectionAndCanReopenSplit(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })

	f := render.New(t)
	pickerFixtureTarget(t, f)
	defer f.Cleanup()
	f.Render(ui.CreateElement(SplitEditor, splitEditorProps{
		Txn: domain.Transaction{
			ID: "split-picker", Amount: money.New(-5000, "USD"),
			Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-5000, "USD")}},
		},
		Categories: []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}},
	}))

	category := byTestID(f, "select", "split-cat-0")
	if category == nil {
		t.Fatal("split category selector did not render")
	}
	category.Change(splitNewCategoryOption)
	f.Stabilize()
	if byTestID(f, "div", "catpick") == nil {
		t.Fatal("choosing New category did not open the category picker")
	}
	byTestID(f, "button", "catpick-close").Click()
	f.Stabilize()

	category = byTestID(f, "select", "split-cat-0")
	if category == nil {
		t.Fatal("category selector disappeared after dismissing the picker")
	}
	if selectedValue := category.Property("value"); selectedValue != "food" {
		t.Fatalf("category select value after dismissal = %#v, want food", selectedValue)
	}
	category.Change(splitNewCategoryOption)
	f.Stabilize()
	if byTestID(f, "div", "catpick") == nil {
		t.Fatal("choosing New category after dismissing it once did not reopen the picker")
	}
}

func TestSplitNewCategoryDismissalRestoresEmptySelection(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })

	f := render.New(t)
	pickerFixtureTarget(t, f)
	defer f.Cleanup()
	f.Render(ui.CreateElement(SplitEditor, splitEditorProps{
		Txn: domain.Transaction{
			ID: "split-picker-empty", Amount: money.New(-5000, "USD"),
			Splits: []domain.CategorySplit{{Amount: money.New(-5000, "USD")}},
		},
		Categories: []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}},
	}))

	category := byTestID(f, "select", "split-cat-0")
	if category == nil {
		t.Fatal("split category selector did not render")
	}
	category.Change(splitNewCategoryOption)
	f.Stabilize()
	if byTestID(f, "div", "catpick") == nil {
		t.Fatal("choosing New category did not open the category picker")
	}
	byTestID(f, "button", "catpick-close").Click()
	f.Stabilize()

	category = byTestID(f, "select", "split-cat-0")
	if category == nil || category.Property("value") != "" {
		t.Fatalf("category select value after dismissing the picker = %v, want the empty category", category)
	}
	category.Change(splitNewCategoryOption)
	f.Stabilize()
	if byTestID(f, "div", "catpick") == nil {
		t.Fatal("choosing New category after dismissing it once did not reopen the picker")
	}
}
