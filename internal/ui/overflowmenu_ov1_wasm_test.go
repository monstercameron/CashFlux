// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestOV1OverflowMenuHidesExtraContentUntilOpen(t *testing.T) {
	f := render.New(t)
	f.Render(OverflowMenu(OverflowMenuProps{
		TriggerTestID: "more",
		Items:         []OverflowMenuItem{{Label: "Rules"}},
		ExtraContent:  Div(Attr("data-testid", "extra-content"), "Saved views"),
	}))

	menu := ov1Menu(f)
	if menu == nil {
		t.Fatal("menu with ExtraContent was not rendered")
	}
	if !strings.Contains(menu.Attr("class"), "hidden-menu") {
		t.Fatalf("closed menu class = %q, want hidden-menu", menu.Attr("class"))
	}
	trigger := ov1Trigger(f)
	if trigger == nil {
		t.Fatal("More trigger was not rendered")
	}
	if got := trigger.Attr("aria-expanded"); got != "false" {
		t.Fatalf("closed trigger aria-expanded = %q, want false", got)
	}

	trigger.Click()
	menu = ov1Menu(f)
	if menu == nil || strings.Contains(menu.Attr("class"), "hidden-menu") {
		t.Fatalf("open menu is missing or hidden: %#v", menu)
	}
	if !strings.Contains(menu.Text(), "Rules") || !strings.Contains(menu.Text(), "Saved views") {
		t.Fatalf("open menu text = %q, want actions and ExtraContent", menu.Text())
	}
	if got := trigger.Attr("aria-expanded"); got != "true" {
		t.Fatalf("open trigger aria-expanded = %q, want true", got)
	}

	trigger.Click()
	menu = ov1Menu(f)
	if menu == nil || !strings.Contains(menu.Attr("class"), "hidden-menu") {
		t.Fatalf("closed-again menu is missing or visible: %#v", menu)
	}
	if got := trigger.Attr("aria-expanded"); got != "false" {
		t.Fatalf("closed-again trigger aria-expanded = %q, want false", got)
	}
}

func ov1Trigger(f *render.Fixture) *render.QueryNode {
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == "more" {
			return node
		}
	}
	return nil
}

func ov1Menu(f *render.Fixture) *render.QueryNode {
	for _, node := range f.AllByTag("div") {
		if node.Attr("role") == "menu" {
			return node
		}
	}
	return nil
}
