// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestR2FEscapeClosesSheetAndKeepsTriggerTarget(t *testing.T) {
	f := render.New(t)
	f.Render(FilterToolbar(FilterToolbarProps{
		FiltersLabel: "Filters",
		FiltersTitle: "Transaction filters",
		FilterSheet:  true,
	}))
	trigger := f.AllByTag("button")[0]
	trigger.Click()
	f.Stabilize()
	var panel *render.QueryNode
	for _, node := range f.AllByTag("div") {
		if node.Attr("id") == "filter-inline-panel" {
			panel = node
			break
		}
	}
	if panel == nil {
		t.Fatal("filter sheet did not open")
	}
	if trigger.Attr("id") == "" {
		t.Fatal("filter trigger has no focus target id")
	}
	panel.Dispatch("onkeydown", render.Event{Key: "Escape"})
	f.Stabilize()
	for _, node := range f.AllByTag("div") {
		// The sheet fades out before it leaves, so it is closed once it is leaving.
		if node.Attr("id") == "filter-inline-panel" && !strings.Contains(node.Attr("class"), "is-leaving") {
			t.Fatal("Escape left the filter sheet open")
		}
	}
}
