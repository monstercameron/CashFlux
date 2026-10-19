// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestQuickAddTV7DCategoryOptionsFollowDirection(t *testing.T) {
	state, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []domain.Category{
		{ID: "groceries", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "salary", Name: "Salary", Kind: domain.KindIncome},
	} {
		if err := state.PutCategory(category); err != nil {
			t.Fatalf("PutCategory(%s): %v", category.ID, err)
		}
	}
	previous := appstate.Default
	appstate.Default = state
	uistate.SetQuickAdd(false)
	t.Cleanup(func() { appstate.Default = previous; uistate.SetQuickAdd(false) })

	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("tv7d-body"), ui.CreateElement(QuickAddHost)))
	uistate.SetQuickAdd(true)
	f.Render(Body(ID("tv7d-body"), ui.CreateElement(QuickAddHost)))
	categorySelect := findAppTestID(f, "select", "txn-add-category")
	if categorySelect == nil {
		t.Fatalf("Quick Add category selector is missing: %q", f.Text())
	}
	if !hasCategoryOption(f, "groceries") || hasCategoryOption(f, "salary") {
		t.Fatalf("Money out should offer expense categories only; form=%q options=%v", f.Text(), categoryOptionValues(f))
	}
	categorySelect.Change("groceries")
	f.Stabilize()

	findAppTestID(f, "button", "txn-add-direction-income").Click()
	f.Stabilize()
	if hasCategoryOption(f, "groceries") || !hasCategoryOption(f, "salary") {
		t.Fatal("Money in should offer income categories only")
	}
	blankSelected := false
	for _, option := range f.AllByTag("option") {
		blankSelected = blankSelected || option.Property("value") == "" && option.Property("selected") == true
	}
	if !blankSelected {
		t.Fatal("changing direction must clear the now-invalid selected category")
	}
}

func hasCategoryOption(f *render.Fixture, value string) bool {
	for _, option := range f.AllByTag("option") {
		if option.Property("value") == value {
			return true
		}
	}
	return false
}

func categoryOptionValues(f *render.Fixture) []string {
	var values []string
	for _, option := range f.AllByTag("option") {
		values = append(values, option.Property("value").(string)+":"+option.Text())
	}
	return values
}

func TestQuickAddTV7DCategoryValidityFollowsDirection(t *testing.T) {
	categories := []domain.Category{
		{ID: "groceries", Kind: domain.KindExpense},
		{ID: "salary", Kind: domain.KindIncome},
	}
	for _, tc := range []struct {
		name, id, direction string
		want                bool
	}{
		{name: "expense in money out", id: "groceries", direction: "Expense", want: true},
		{name: "income in money out", id: "salary", direction: "Expense"},
		{name: "income in money in", id: "salary", direction: "Income", want: true},
		{name: "expense in money in", id: "groceries", direction: "Income"},
		{name: "unknown category", id: "missing", direction: "Expense"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := quickAddCategoryIsValid(categories, tc.id, tc.direction); got != tc.want {
				t.Fatalf("quickAddCategoryIsValid(%q, %q) = %v, want %v", tc.id, tc.direction, got, tc.want)
			}
		})
	}
}
