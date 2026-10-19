// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR8SFilterGroupShowAllExpandsInPlace(t *testing.T) {
	f := render.New(t)
	options := make([]uiw.SelectOption, 14)
	for i := range options {
		options[i] = uiw.SelectOption{Value: string(rune('a' + i)), Label: "Option " + string(rune('A'+i))}
	}
	f.Render(uic.CreateElement(filterGroup, filterGroupProps{
		Label: "Person",
		Field: txnfilter.FieldMember,
		Opts:  options,
	}))

	var expand *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Attr("data-testid") == "filter-showall-"+string(txnfilter.FieldMember) {
			expand = button
			break
		}
	}
	if expand == nil || expand.Text() != "Show all 14" || expand.Attr("aria-expanded") != "false" {
		t.Fatalf("collapsed filter group toggle = %#v, want Show all 14 collapsed", expand)
	}
	if got := len(f.AllByTag("button")); got != 1 {
		t.Fatalf("collapsed group rendered %d buttons, want only its expansion control", got)
	}

	expand.Click()
	f.Stabilize()
	if got := len(f.AllByTag("button")); got != 15 {
		t.Fatalf("expanded group rendered %d buttons, want 14 options and collapse control", got)
	}
	var collapse *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Attr("data-testid") == "filter-showall-"+string(txnfilter.FieldMember) {
			collapse = button
			break
		}
	}
	if collapse == nil || collapse.Text() != "Show less" || collapse.Attr("aria-expanded") != "true" {
		t.Fatalf("expanded filter group toggle = %#v, want Show less expanded", collapse)
	}
}
