// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTXU6FilterToolbarPrimaryRowAndKeyboardOrder(t *testing.T) {
	f := render.New(t)
	f.Render(FilterToolbar(FilterToolbarProps{
		SearchLabel:  "Search transactions",
		FiltersLabel: "Filters",
		FiltersTitle: "Transaction filters",
		SearchAddon:  Button(Type("button"), Attr("data-testid", "date-scope"), "Date scope"),
		PrimaryActions: []uic.Node{
			Button(Type("button"), Attr("data-testid", "add"), "Add transaction"),
		},
		Actions: []uic.Node{
			Button(Type("button"), Attr("data-testid", "secondary"), "Secondary action"),
		},
	}))

	inputs := f.AllByTag("input")
	if len(inputs) == 0 || inputs[0].Attr("role") != "searchbox" {
		t.Fatal("search field is not the first keyboard control")
	}
	buttons := f.AllByTag("button")
	want := []string{"date-scope", "", "add", "secondary"}
	if len(buttons) != len(want) {
		t.Fatalf("toolbar buttons = %d, want %d", len(buttons), len(want))
	}
	for i, id := range want {
		if got := buttons[i].Attr("data-testid"); got != id {
			t.Fatalf("keyboard button %d test id = %q, want %q", i, got, id)
		}
	}

	rows := f.AllByTag("div")
	var primary string
	for _, row := range rows {
		if row.Attr("class") == "filter-toolbar-primary" {
			primary = row.Text()
			break
		}
	}
	if primary == "" {
		t.Fatal("primary toolbar row was not rendered")
	}
	last := -1
	for _, part := range []string{"Date scope", "Filters", "Add transaction"} {
		at := strings.Index(primary, part)
		if at < 0 || at <= last {
			t.Errorf("primary row %q does not place %q after the previous control", primary, part)
		}
		last = at
	}
}

func TestTXU6OverflowMenuCanHostOptionalViewsGroup(t *testing.T) {
	f := render.New(t)
	f.Render(OverflowMenu(OverflowMenuProps{
		TriggerText:   "More",
		TriggerTestID: "more",
		Items:         []OverflowMenuItem{{Label: "Rules", TestID: "rules"}},
		ExtraContent: Div(css.Class("saved-views-embedded"), Attr("role", "group"),
			Button(Type("button"), Attr("role", "menuitem"), Attr("data-testid", "txn-views-btn"), "Saved views")),
	}))

	menu := ov1Menu(f)
	if menu == nil {
		t.Fatal("More menu was not rendered")
	}
	if !strings.Contains(menu.Attr("class"), "hidden-menu") {
		t.Fatalf("closed menu class = %q, want hidden-menu", menu.Attr("class"))
	}
	if strings.Contains(menu.Text(), "Rules") {
		t.Fatalf("closed menu rendered its Rules action: %q", menu.Text())
	}
	trigger := ov1Trigger(f)
	if trigger == nil || trigger.Attr("aria-expanded") != "false" {
		t.Fatalf("closed More trigger missing or expanded: %#v", trigger)
	}
	trigger.Click()
	menu = ov1Menu(f)
	if menu == nil || strings.Contains(menu.Attr("class"), "hidden-menu") {
		t.Fatalf("opened menu is missing or hidden: %#v", menu)
	}
	if !strings.Contains(menu.Text(), "Rules") || !strings.Contains(menu.Text(), "Saved views") {
		t.Fatalf("More menu text = %q, want both Rules and Saved views", menu.Text())
	}
	if menu.Attr("class") == "" {
		t.Fatal("menu container lost its existing class")
	}
}

func TestR1IOverflowMenuOpensWithoutEffectPanicAndMarksOwner(t *testing.T) {
	f := render.New(t)
	f.Render(OverflowMenu(OverflowMenuProps{
		TriggerTestID: "row-actions",
		Items:         []OverflowMenuItem{{Label: "Edit"}},
	}))

	var trigger *render.QueryNode
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == "row-actions" {
			trigger = node
			break
		}
	}
	if trigger == nil {
		t.Fatal("row action trigger was not rendered")
	}
	trigger.Click()
	if got := trigger.Attr("aria-expanded"); got != "true" {
		t.Fatalf("trigger aria-expanded = %q, want true", got)
	}

	var wrapper *render.QueryNode
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "overflow-menu-open") {
			wrapper = node
			break
		}
	}
	if wrapper == nil {
		t.Fatal("open menu wrapper does not expose the owning-row state")
	}
	if !strings.Contains(wrapper.Text(), "Edit") {
		t.Fatalf("open menu text = %q, want Edit", wrapper.Text())
	}
}
