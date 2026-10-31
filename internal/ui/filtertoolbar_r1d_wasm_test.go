// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestR1DEscapeClearsSearchImmediately(t *testing.T) {
	f := render.New(t)
	cleared := 0
	f.Render(FilterToolbar(FilterToolbarProps{
		Search:        "coffee",
		SearchLabel:   "Search transactions",
		FiltersLabel:  "Filters",
		FiltersTitle:  "Transaction filters",
		OnClearSearch: func() { cleared++ },
	}))
	inputs := f.AllByTag("input")
	if len(inputs) == 0 {
		t.Fatal("search input was not rendered")
	}
	inputs[0].Dispatch("onkeydown", render.Event{Key: "Escape"})
	if cleared != 1 {
		t.Fatalf("Escape clear callback ran %d times, want once", cleared)
	}
}

// TestR1DFilterSheetIsOptIn checks that the inline disclosure stays the default and that the
// sheet variant is portaled out of the toolbar. The shared Overlay renders through a portal to
// the page body, which the wasm test renderer does not have, so the sheet is absent from the
// render tree here and its footer and Escape behaviour are checked in the browser.
func TestR1DFilterSheetIsOptIn(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sheet      bool
		wantInline bool
	}{
		{name: "existing inline default", wantInline: true},
		{name: "opt-in right sheet is portaled", sheet: true, wantInline: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(FilterToolbar(FilterToolbarProps{
				SearchLabel:  "Search",
				FiltersLabel: "Filters",
				FiltersTitle: "Filters",
				FilterSheet:  tc.sheet,
			}))
			f.AllByTag("button")[0].Click()
			f.Stabilize()
			inline := false
			for _, node := range f.AllByTag("div") {
				if node.Attr("id") == "filter-inline-panel" {
					inline = true
				}
			}
			if inline != tc.wantInline {
				t.Fatalf("inline filter panel present = %t, want %t", inline, tc.wantInline)
			}
		})
	}
}
