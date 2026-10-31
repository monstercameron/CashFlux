// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

// TestR3SFilterSheetIsNotRenderedInline checks that the Transactions sheet leaves the toolbar's
// render tree, so a transformed bento tile cannot become its containing block. Its footer
// buttons only exist inside the portal, so they are checked in the browser.
func TestR3SFilterSheetIsNotRenderedInline(t *testing.T) {
	f := render.New(t)
	f.Render(FilterToolbar(FilterToolbarProps{
		FiltersLabel: "Filters",
		FiltersTitle: "Filters",
		FilterSheet:  true,
		OnClearAll:   func() {},
	}))
	f.AllByTag("button")[0].Click()
	f.Stabilize()
	for _, node := range f.AllByTag("div") {
		if node.Attr("id") == "filter-inline-panel" {
			t.Fatal("the filter sheet rendered inline; it must portal to the page body")
		}
	}
}
