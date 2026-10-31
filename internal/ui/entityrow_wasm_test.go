// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestEntityRowValueSitsBetweenTextAndActions(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(EntityRow(EntityRowProps{
		Leading: Span(Attr("data-testid", "lead"), "L"),
		Title:   "Checking",
		Meta:    []string{"Bank"},
		Value:   Span(Attr("data-testid", "val"), "12.00"),
		Actions: []uic.Node{Button(Attr("data-testid", "act"), "Edit")},
	}))
	val := uiNodeByTestID(f, "val")
	if val == nil {
		t.Fatal("EntityRow did not render its Value")
	}
	wrap := uiParentOf(f, val)
	if wrap == nil || !containsClass(wrap.Attr("class"), "row-value") {
		t.Fatal("Value is not wrapped in .row-value")
	}
	row := uiParentOf(f, wrap)
	if row == nil || !containsClass(row.Attr("class"), "row") {
		t.Fatal("Value is not a direct child slot of the .row")
	}
	kids := row.Children()
	if len(kids) != 4 {
		t.Fatalf("row children = %d, want leading, text, value, action", len(kids))
	}
	if !containsClass(kids[1].Attr("class"), "row-main") || !containsClass(kids[2].Attr("class"), "row-value") || kids[3].Attr("data-testid") != "act" {
		t.Fatal("slot order is not leading, row-main, row-value, actions")
	}
}

func TestEntityRowWithoutValueRendersUnchanged(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(EntityRow(EntityRowProps{
		Title:   "Savings",
		Actions: []uic.Node{Button(Attr("data-testid", "act"), "Edit")},
	}))
	var row *render.QueryNode
	for _, div := range f.AllByTag("div") {
		if containsClass(div.Attr("class"), "row-value") {
			t.Fatal("a nil Value must render no .row-value element")
		}
		if div.Attr("class") == "row" {
			row = div
		}
	}
	if row == nil || len(row.Children()) != 2 {
		t.Fatal("a row with no leading and no value must hold only the text block and the action")
	}
}
