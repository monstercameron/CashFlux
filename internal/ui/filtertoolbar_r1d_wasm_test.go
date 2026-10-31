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
