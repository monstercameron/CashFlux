// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestTV7BMoreCategoriesIsKeyboardActionAndRevealsFullList(t *testing.T) {
	categories := []txnCategoryTotal{
		{ID: "a", Name: "Housing", Minor: 6000},
		{ID: "b", Name: "Food", Minor: 5000},
		{ID: "c", Name: "Travel", Minor: 4000},
		{ID: "d", Name: "Other", Minor: 3000},
		{ID: "e", Name: "Health", Minor: 2000},
	}
	f := render.New(t)
	f.Render(txnCategoryBreakdown(categories, "", 20000))
	more := r2aByTestID(f, "button", "txn-category-more")
	if more == nil || more.Attr("aria-expanded") != "false" || more.Attr("aria-haspopup") != "dialog" {
		t.Fatalf("more categories control is not an accessible button: %v", more)
	}
	segment := findCategoryButton(f, "txn-category-segment", "a")
	if segment == nil || segment.Attr("aria-hidden") != "true" || segment.Attr("tabindex") != "-1" {
		t.Errorf("bar segment should be decorative and not a tab stop: %#v", segment)
	}
	more.Click()
	f.Render(txnCategoryBreakdown(categories, "", 20000))
	if expanded := r2aByTestID(f, "button", "txn-category-more"); expanded == nil || expanded.Attr("aria-expanded") != "true" {
		t.Fatalf("disclosure did not open: %v", expanded)
	}
	if popover := r2aByTestID(f, "div", "txn-category-popover"); popover == nil || popover.Attr("role") != "dialog" {
		t.Fatal("more categories should open a labeled popover")
	}
	if item := findCategoryButton(f, "txn-category-item", "e"); item == nil {
		t.Fatal("category popover omitted Health")
	}
}

func TestTV7BCategoryLabelsIncludeRealOtherCategory(t *testing.T) {
	f := render.New(t)
	f.Render(txnCategoryBreakdown([]txnCategoryTotal{{ID: "other", Name: "Other", Minor: 1234}}, "", 1234))
	if item := findCategoryButton(f, "txn-category-item", "other"); item == nil || item.Text() == "" {
		t.Fatalf("actual Other category has no readable amount: %v", item)
	}
}

func TestTV7BDisclosureKeepsHooksStableAsCategoriesChange(t *testing.T) {
	categories := []txnCategoryTotal{
		{ID: "a", Name: "Housing", Minor: 6000},
		{ID: "b", Name: "Food", Minor: 5000},
		{ID: "c", Name: "Travel", Minor: 4000},
		{ID: "d", Name: "Health", Minor: 3000},
	}
	f := render.New(t)
	defer f.Cleanup()
	for _, count := range []int{1, 4, 2, 4} {
		f.Render(txnCategoryBreakdown(categories[:count], "", 18000))
		more := r2aByTestID(f, "button", "txn-category-more")
		if count <= 3 {
			if more != nil {
				t.Fatal("short category list has an unnecessary disclosure")
			}
			continue
		}
		if more == nil {
			t.Fatal("long category list lost its disclosure after changing size")
		}
		more.Click()
		if r2aByTestID(f, "button", "txn-category-more").Attr("aria-expanded") != "true" {
			t.Fatal("disclosure handler changed when the category count changed")
		}
		if findCategoryButton(f, "txn-category-item", "d") == nil {
			t.Fatal("expanded list omitted the fourth category")
		}
		r2aByTestID(f, "button", "txn-category-more").Click()
	}
}

func TestTV7BFigureAccessibleNameSeparatesAmountAndLabel(t *testing.T) {
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 1, Total: 1}))
	spent := r2aByTestID(f, "button", "txn-count-spent")
	if spent == nil || spent.Attr("aria-label") == "" || spent.Attr("aria-label")[len(spent.Attr("aria-label"))-5:] == "Spent" {
		t.Fatalf("Spent figure needs a readable name with its label and amount separated: %v", spent)
	}
}
