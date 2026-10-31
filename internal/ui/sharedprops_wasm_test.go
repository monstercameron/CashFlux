// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"slices"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestEntityRowRootCarriesTestIDAndAttrs(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(EntityRow(EntityRowProps{
		Title: "Checking", TestID: "row-checking",
		Attrs: []any{Attr("data-kind", "account"), Attr("id", "row-1")},
	}))
	row := uiNodeByTestID(f, "row-checking")
	if row == nil {
		t.Fatal("EntityRow root has no data-testid")
	}
	if !containsClass(row.Attr("class"), "row") {
		t.Fatalf("root class = %q, want row", row.Attr("class"))
	}
	if row.Attr("data-kind") != "account" || row.Attr("id") != "row-1" {
		t.Fatalf("root attrs = kind %q id %q, want the extra attributes", row.Attr("data-kind"), row.Attr("id"))
	}
}

func TestDisclosureOnToggleReportsEachNewState(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	var seen []bool
	f.Render(Disclosure(DisclosureProps{
		ID: "more", Summary: Span("More"), Body: P("Extra"), TestID: "more-toggle",
		OnToggle: func(open bool) { seen = append(seen, open) },
	}))
	buttonByTestID(f, "more-toggle").Click()
	f.Stabilize()
	buttonByTestID(f, "more-toggle").Click()
	f.Stabilize()
	if !slices.Equal(seen, []bool{true, false}) {
		t.Fatalf("OnToggle states = %v, want [true false]", seen)
	}
	if buttonByTestID(f, "more-toggle").Attr("aria-expanded") != "false" {
		t.Fatal("the Disclosure's own state must stay the source of truth after OnToggle")
	}
}

func TestOverflowMenuItemRendersTitleTooltip(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(OverflowMenu(OverflowMenuProps{
		TriggerTestID: "menu-trigger",
		Items: []OverflowMenuItem{
			{Label: "Reconcile", TestID: "menu-reconcile", Title: "Match the account to a statement"},
			{Label: "Edit", TestID: "menu-edit"},
		},
	}))
	buttonByTestID(f, "menu-trigger").Click()
	f.Stabilize()
	if got := buttonByTestID(f, "menu-reconcile").Attr("title"); got != "Match the account to a statement" {
		t.Fatalf("entry title = %q, want the tooltip", got)
	}
	if got := buttonByTestID(f, "menu-edit").Attr("title"); got != "" {
		t.Fatalf("entry without a Title has title %q, want none", got)
	}
}

func TestSelectInputRendersID(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Field(FieldProps{
		ID: "from-pick", Label: "From",
		Control: func(a FieldA11y) uic.Node {
			return SelectInput(SelectInputProps{
				ID: a.ID, TestID: "from-select", Selected: "a",
				Options: []SelectOption{{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}},
			})
		},
	}))
	for _, sel := range f.AllByTag("select") {
		if sel.Attr("data-testid") == "from-select" {
			if sel.Attr("id") != "from-pick" {
				t.Fatalf("select id = %q, want from-pick", sel.Attr("id"))
			}
			return
		}
	}
	t.Fatal("SelectInput was not rendered")
}

func TestMoneyInputRendersIDTestIDPlaceholderAndExtra(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(MoneyInput(MoneyInputProps{
		Currency: "USD", ID: "keep-amt", TestID: "keep", Placeholder: "300.00",
		Extra: []any{Attr("inputmode", "decimal")},
	}))
	inputs := f.AllByTag("input")
	if len(inputs) != 1 {
		t.Fatalf("MoneyInput rendered %d inputs, want 1", len(inputs))
	}
	in := inputs[0]
	for attr, want := range map[string]string{
		"id": "keep-amt", "data-testid": "keep", "placeholder": "300.00", "inputmode": "decimal", "type": "number", "step": "0.01",
	} {
		if got := in.Attr(attr); got != want {
			t.Errorf("input %s = %q, want %q", attr, got, want)
		}
	}
}
