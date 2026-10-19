// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestTV8CCategoryPopoverKeepsEveryCategoryReadable(t *testing.T) {
	categories := []txnCategoryTotal{
		{ID: "a", Name: "Housing", Minor: 6000},
		{ID: "b", Name: "Food", Minor: 5000},
		{ID: "c", Name: "Travel", Minor: 4000},
		{ID: "d", Name: "Health", Minor: 3000},
		{ID: "e", Name: "Other", Minor: 2000},
		{ID: "f", Name: "Care", Minor: 1000},
		{ID: "g", Name: "Utilities", Minor: 500},
	}
	f := render.New(t)
	f.Render(txnCategoryBreakdown(categories, "", 21500))
	other := findCategoryButton(f, "txn-category-segment", "__other__")
	if other == nil || other.Attr("title") != "Other $15.00" {
		t.Fatalf("bar should aggregate categories beyond five into Other: %v", other)
	}
	more := r2aByTestID(f, "button", "txn-category-more")
	if more == nil {
		t.Fatal("category list needs a more button")
	}
	if findCategoryButton(f, "txn-category-item", "d") != nil {
		t.Fatal("hidden categories should not be expanded inline")
	}
	more.Click()
	f.Render(txnCategoryBreakdown(categories, "", 21500))
	popover := r2aByTestID(f, "div", "txn-category-popover")
	if popover == nil || popover.Attr("role") != "dialog" {
		t.Fatal("more button did not open a category popover")
	}
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		if findCategoryButton(f, "txn-category-item", id) == nil {
			t.Errorf("popover omitted category %q", id)
		}
	}
	if !strings.Contains(popover.Text(), "27.9%") {
		t.Fatalf("popover omitted a category share: %q", popover.Text())
	}
}

func TestTV8CFactsStayOutsideTheSummaryBandAndCountOnce(t *testing.T) {
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 14, Total: 14}))
	line := r2aByTestID(f, "div", "txn-count-line")
	if line == nil {
		t.Fatal("summary line is missing")
	}
	band := r2aByTestID(f, "div", "txn-count-band")
	facts := r2aByTestID(f, "div", "txn-count-visible-facts")
	if band == nil {
		t.Fatal("summary band is missing")
	}
	if facts == nil {
		t.Fatalf("facts row is missing; rendered text: %q", f.Text())
	}
	if strings.Contains(band.Text(), "14 transactions") {
		t.Fatal("facts row must sit below the summary band")
	}
	status := r2aByTestID(f, "span", "txn-count-line")
	if status == nil || strings.Count(status.Text(), "14 transactions") != 1 {
		t.Fatalf("live status repeats the transaction count: %q", status.Text())
	}
}
