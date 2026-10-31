// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestR3SFilterSheetFooterClearsAndCloses(t *testing.T) {
	f := render.New(t)
	cleared := 0
	f.Render(FilterToolbar(FilterToolbarProps{
		FiltersLabel: "Filters",
		FiltersTitle: "Filters",
		FilterSheet:  true,
		OnClearAll:   func() { cleared++ },
	}))
	f.AllByTag("button")[0].Click()
	f.Stabilize()

	var clearButton, doneButton *render.QueryNode
	for _, button := range f.AllByTag("button") {
		switch button.Attr("data-testid") {
		case "filter-sheet-clear-all":
			clearButton = button
		case "filter-sheet-done":
			doneButton = button
		}
	}
	if clearButton == nil || doneButton == nil {
		t.Fatal("filter sheet footer is missing Clear all or Done")
	}
	clearButton.Click()
	if cleared != 1 {
		t.Fatalf("Clear all callback ran %d times, want once", cleared)
	}
	doneButton.Click()
	f.Stabilize()
	for _, node := range f.AllByTag("div") {
		if node.Attr("id") == "filter-inline-panel" && !strings.Contains(node.Attr("class"), "is-leaving") {
			t.Fatal("Done left the filter sheet open")
		}
	}
}
