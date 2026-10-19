// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestUXT6FilterSheetHasOneMarkedScrollOwner(t *testing.T) {
	f := render.New(t)
	f.Render(FilterToolbar(FilterToolbarProps{
		FiltersLabel: "Filters",
		FiltersTitle: "Transaction filters",
		FilterSheet:  true,
	}))
	f.AllByTag("button")[0].Click()
	f.Stabilize()
	for _, node := range f.AllByTag("div") {
		if node.Attr("class") == "filter-inline-body" && node.Attr("data-scroll-owner") != "sheet" {
			t.Fatalf("filter body scroll owner = %q, want sheet", node.Attr("data-scroll-owner"))
		}
	}
}
