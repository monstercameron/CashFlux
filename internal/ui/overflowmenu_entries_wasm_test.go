// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"slices"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/interop"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

// TestOverflowSubmenuKeepsParentOrderAndVisibleAccessibleNames checks that a submenu does not
// reorder the parent menu, and that menu items are named by their visible text only.
func TestOverflowSubmenuKeepsParentOrderAndVisibleAccessibleNames(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(OverflowMenu(OverflowMenuProps{
		TriggerTestID: "menu-trigger",
		Items: []OverflowMenuItem{
			{Label: "Split…", Section: "Organize", TestID: "menu-split"},
			{Label: "Link to…", Section: "Link", TestID: "menu-link", Submenu: []OverflowMenuItem{{Label: "Mark as bill payment", TestID: "menu-bill"}}},
			{Label: "Exclude", Section: "Reports", TestID: "menu-exclude"},
			{Label: "Delete", Section: "Remove", Danger: true, TestID: "menu-delete"},
		},
	}))
	trigger := buttonByTestID(f, "menu-trigger")
	trigger.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if buttonByTestID(f, "menu-split") == nil {
		t.Fatal("Enter on the focused overflow trigger did not open its menu")
	}
	trigger.Click()
	f.Stabilize()
	trigger.Dispatch("onkeydown", render.Event{Key: " "})
	if buttonByTestID(f, "menu-split") == nil {
		t.Fatal("Space on the focused overflow trigger did not open its menu")
	}
	order := func() []string {
		var labels []string
		for _, button := range f.AllByTag("button") {
			// The linked submenu's item is also a menuitem. It is left out of the parent order.
			if button.Attr("role") == "menuitem" && button.Attr("data-testid") != "menu-bill" {
				labels = append(labels, button.Text())
			}
		}
		return labels
	}
	want := []string{"Split…", "Link to…", "Exclude", "Delete"}
	assertLabels := func(got []string) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Fatalf("parent menu items = %#v, want %#v", got, want)
		}
	}
	assertLabels(order())
	link := buttonByTestID(f, "menu-link")
	if got := link.Attr("aria-label"); got != "" {
		t.Fatalf("Link accessible label = %q, want visible text only", got)
	}
	link.Dispatch("onkeydown", render.Event{Key: "ArrowRight"})
	if buttonByTestID(f, "menu-bill") == nil {
		t.Fatal("Right did not open the linked submenu")
	}
	assertLabels(order())
	if got := buttonByTestID(f, "menu-delete").Text(); got != "Delete" {
		t.Fatalf("last parent action = %q, want Delete", got)
	}
	if got := buttonByTestID(f, "menu-delete").Attr("aria-label"); got != "" {
		t.Fatalf("Delete accessible label = %q, want visible text only", got)
	}
	// The Remove section heading and the Delete button must both render.
	removeHeading, deleteButton := false, false
	for _, node := range f.AllByTag("div") {
		if node.Text() == "Remove" && node.Attr("class") == "add-menu-section" {
			removeHeading = true
		}
	}
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == "menu-delete" {
			deleteButton = true
		}
	}
	if !removeHeading || !deleteButton {
		t.Fatal("Delete's Remove separator or button is missing")
	}
}

// buttonByTestID returns the first button with the given data-testid, or nil when there is none.
func buttonByTestID(f *render.Fixture, id string) *render.QueryNode {
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == id {
			return node
		}
	}
	return nil
}

// TestMenuFocusDoesNotScroll checks that focusing a menu item asks the browser not to scroll.
func TestMenuFocusDoesNotScroll(t *testing.T) {
	item := fakeObject(t)
	called := false
	preventScroll := false
	fakeMethod(t, item, "focus", func(args ...interop.Value) any {
		called = true
		if len(args) > 0 {
			preventScroll = args[0].Get("preventScroll").Bool()
		}
		return nil
	})
	focusMenuItem(item)
	if !called || !preventScroll {
		t.Fatal("menu focus did not request preventScroll")
	}
}
