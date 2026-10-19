// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestR3KClosedOverflowMenuDoesNotMountMenuItems(t *testing.T) {
	f := render.New(t)
	f.Render(OverflowMenu(OverflowMenuProps{
		TriggerTestID: "row-menu",
		Items: []OverflowMenuItem{
			{Label: "Edit", TestID: "edit"},
			{Label: "Delete", TestID: "delete"},
		},
	}))
	if got := overflowTestMenuItems(f); len(got) != 0 {
		t.Fatalf("closed menu mounted %d menuitems, want none", len(got))
	}
	triggers := f.AllByTag("button")
	if len(triggers) != 1 || triggers[0].Attr("data-testid") != "row-menu" {
		t.Fatalf("closed menu buttons = %d, want only its trigger", len(triggers))
	}
	triggers[0].Click()
	if got := overflowTestMenuItems(f); len(got) != 2 {
		t.Fatalf("open menu mounted %d menuitems, want 2", len(got))
	}
}

func overflowTestMenuItems(f *render.Fixture) []*render.QueryNode {
	var items []*render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Attr("role") == "menuitem" {
			items = append(items, button)
		}
	}
	return items
}
