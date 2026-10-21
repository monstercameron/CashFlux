// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestUXT4OverflowSubmenuKeepsParentOrderAndVisibleAccessibleNames(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(OverflowMenu(OverflowMenuProps{
		TriggerTestID: "uxt4-trigger",
		Items: []OverflowMenuItem{
			{Label: "Split…", Section: "Organize", TestID: "uxt4-split"},
			{Label: "Link to…", Section: "Link", TestID: "uxt4-link", Submenu: []OverflowMenuItem{{Label: "Mark as bill payment", TestID: "uxt4-bill"}}},
			{Label: "Exclude", Section: "Reports", TestID: "uxt4-exclude"},
			{Label: "Delete", Section: "Remove", Danger: true, TestID: "uxt4-delete"},
		},
	}))
	trigger := uxt4MenuByTestID(f, "uxt4-trigger")
	trigger.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if uxt4MenuByTestID(f, "uxt4-split") == nil {
		t.Fatal("Enter on the focused overflow trigger did not open its menu")
	}
	trigger.Click()
	f.Stabilize()
	trigger.Dispatch("onkeydown", render.Event{Key: " "})
	if uxt4MenuByTestID(f, "uxt4-split") == nil {
		t.Fatal("Space on the focused overflow trigger did not open its menu")
	}
	order := func() []string {
		var labels []string
		for _, button := range f.AllByTag("button") {
			if button.Attr("role") == "menuitem" && button.Attr("data-testid") != "uxt4-bill" {
				labels = append(labels, button.Text())
			}
		}
		return labels
	}
	want := []string{"Split…", "Link to…", "Exclude", "Delete"}
	assertLabels := func(got []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("parent menu items = %#v, want %#v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("parent menu items = %#v, want %#v", got, want)
			}
		}
	}
	assertLabels(order())
	link := uxt4MenuByTestID(f, "uxt4-link")
	if got := link.Attr("aria-label"); got != "" {
		t.Fatalf("Link accessible label = %q, want visible text only", got)
	}
	link.Dispatch("onkeydown", render.Event{Key: "ArrowRight"})
	if uxt4MenuByTestID(f, "uxt4-bill") == nil {
		t.Fatal("Right did not open the linked submenu")
	}
	assertLabels(order())
	if got := uxt4MenuByTestID(f, "uxt4-delete").Text(); got != "Delete" {
		t.Fatalf("last parent action = %q, want Delete", got)
	}
	if got := uxt4MenuByTestID(f, "uxt4-delete").Attr("aria-label"); got != "" {
		t.Fatalf("Delete accessible label = %q, want visible text only", got)
	}
	removeHeading, deleteButton := -1, -1
	for i, node := range f.AllByTag("div") {
		if node.Text() == "Remove" && node.Attr("class") == "add-menu-section" {
			removeHeading = i
		}
	}
	for i, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == "uxt4-delete" {
			deleteButton = i
		}
	}
	if removeHeading < 0 || deleteButton < 0 {
		t.Fatal("Delete's Remove separator or button is missing")
	}
}

func uxt4MenuByTestID(f *render.Fixture, id string) *render.QueryNode {
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == id {
			return node
		}
	}
	return nil
}

func TestUXT4MenuFocusDoesNotScroll(t *testing.T) {
	called := false
	var options js.Value
	focus := js.FuncOf(func(_ js.Value, args []js.Value) any {
		called = true
		if len(args) > 0 {
			options = args[0]
		}
		return nil
	})
	defer focus.Release()
	item := js.Global().Get("Object").New()
	item.Set("focus", focus)
	focusMenuItem(item)
	if !called || !options.Truthy() || !options.Get("preventScroll").Bool() {
		t.Fatal("menu focus did not request preventScroll")
	}
}
