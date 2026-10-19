// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
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

func TestR1DFilterSheetIsOptIn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		sheet bool
		want  string
	}{
		{name: "existing inline default", want: "filter-inline-panel"},
		{name: "opt-in right sheet", sheet: true, want: "filter-inline-panel filter-sheet"},
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
			found := false
			for _, node := range f.AllByTag("div") {
				if node.Attr("id") == "filter-inline-panel" {
					found = true
					if got := node.Attr("class"); got != tc.want {
						t.Fatalf("panel class = %q, want %q", got, tc.want)
					}
				}
			}
			if !found {
				t.Fatal("filter panel did not open")
			}
		})
	}
}
