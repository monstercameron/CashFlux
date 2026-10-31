// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"
	"testing"

	sh "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func renderPickerForTest(f *render.Fixture, props PickerProps) {
	f.Render(sh.Fragment(sh.Div(sh.Attr("id", "picker-test-portal")), uic.CreateElement(pickerComponent, pickerComponentProps{Props: props, PortalTarget: "#picker-test-portal"})))
}

func renderMultiPickerForTest(f *render.Fixture, props MultiPickerProps) {
	f.Render(sh.Fragment(sh.Div(sh.Attr("id", "picker-test-portal")), uic.CreateElement(multiPickerComponent, multiPickerComponentProps{Props: props, PortalTarget: "#picker-test-portal"})))
}

type pickerTestBoundaryProps struct {
	Child uic.Node
	OnKey func()
}

func pickerTestBoundary(props pickerTestBoundaryProps) uic.Node {
	onKey := uic.UseEvent(func(uic.KeyboardEvent) {
		if props.OnKey != nil {
			props.OnKey()
		}
	})
	return sh.Div(sh.OnKeyDown(onKey), props.Child)
}

func TestPickerKeyboardNavigationAndExplicitCommit(t *testing.T) {
	var committed []string
	f := render.New(t)
	renderPickerForTest(f, PickerProps{
		ID: "category-picker", Label: "Category", Placeholder: "Choose a category", Searchable: true,
		Options:  []PickerOption{{Value: "", Label: "Category"}, {Value: "food", Label: "Food"}, {Value: "rent", Label: "Rent"}},
		OnCommit: func(value string) { committed = append(committed, value) },
	})
	trigger := f.ByID("category-picker")
	if !trigger.Exists() {
		t.Fatal("picker trigger was not rendered")
	}
	trigger.Dispatch("onkeydown", render.Event{Key: "ArrowDown"})
	search := f.ByID("category-picker-search")
	if !search.Exists() {
		t.Fatal("opening the picker did not show its search input")
	}
	if got := search.Attr("aria-controls"); got != "category-picker-listbox" {
		t.Fatalf("search controls %q, want category listbox", got)
	}
	if got := search.Attr("aria-label"); got != "Search Category" {
		t.Fatalf("search accessible name %q, want Search Category", got)
	}
	if got := f.ByID("category-picker-option-0").Text(); got != "Food" {
		t.Fatalf("first option is %q, want Food (no placeholder option)", got)
	}
	search.Dispatch("onkeydown", render.Event{Key: "ArrowDown"})
	if got := f.ByID("category-picker-option-1").Attr("aria-selected"); got != "true" {
		t.Fatalf("ArrowDown selected state = %q, want true", got)
	}
	if len(committed) != 0 {
		t.Fatalf("arrow navigation committed %v", committed)
	}
	search.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if len(committed) != 1 || committed[0] != "rent" {
		t.Fatalf("Enter committed %v, want exactly [rent]", committed)
	}
	if f.ByID("category-picker-search").Exists() {
		t.Fatal("picker remained open after commit")
	}
}

func TestPickerEscapeClosesAndReturnsFocusWithoutCommit(t *testing.T) {
	previousDocument := js.Global().Get("document")
	var focused string
	searchFocus := js.FuncOf(func(js.Value, []js.Value) any { focused = "search"; return nil })
	triggerFocus := js.FuncOf(func(js.Value, []js.Value) any { focused = "trigger"; return nil })
	getElementByID := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return js.Null()
		}
		element := js.Global().Get("Object").New()
		switch args[0].String() {
		case "person-picker-search":
			element.Set("focus", searchFocus)
		case "person-picker":
			element.Set("focus", triggerFocus)
		}
		return element
	})
	document := js.Global().Get("Object").New()
	document.Set("getElementById", getElementByID)
	js.Global().Set("document", document)
	t.Cleanup(func() {
		js.Global().Set("document", previousDocument)
		searchFocus.Release()
		triggerFocus.Release()
		getElementByID.Release()
	})
	called := 0
	propagated := 0
	f := render.New(t)
	picker := uic.CreateElement(pickerComponent, pickerComponentProps{Props: PickerProps{
		ID: "person-picker", Label: "Person", Placeholder: "Choose a person", Searchable: true,
		Options:  []PickerOption{{Value: "m1", Label: "Morgan"}, {Value: "m2", Label: "Taylor"}},
		OnCommit: func(string) { called++ },
	}, PortalTarget: "#picker-test-portal"})
	f.Render(sh.Fragment(
		sh.Div(sh.Attr("id", "picker-test-portal")),
		uic.CreateElement(pickerTestBoundary, pickerTestBoundaryProps{
			Child: picker, OnKey: func() { propagated++ },
		}),
	))
	f.ByID("person-picker").Click()
	if focused != "search" {
		t.Fatalf("opening picker focused %q, want search", focused)
	}
	search := f.ByID("person-picker-search")
	search.Dispatch("onkeydown", render.Event{Key: "ArrowDown"})
	search.Dispatch("onkeydown", render.Event{Key: "Escape"})
	if called != 0 {
		t.Fatalf("Escape committed a value %d times", called)
	}
	if propagated != 0 {
		t.Fatalf("Escape propagated to the parent %d times", propagated)
	}
	if f.ByID("person-picker-search").Exists() {
		t.Fatal("Escape did not close picker")
	}
	if focused != "trigger" {
		t.Fatalf("Escape returned focus to %q, want trigger", focused)
	}
}

func TestPickerOptionClickCommitsOnce(t *testing.T) {
	var committed []string
	f := render.New(t)
	renderPickerForTest(f, PickerProps{
		ID: "click-picker", Label: "Person", Placeholder: "Choose a person", Searchable: true,
		Options:  []PickerOption{{Value: "m1", Label: "Morgan"}},
		OnCommit: func(value string) { committed = append(committed, value) },
	})
	f.ByID("click-picker").Click()
	f.ByID("click-picker-option-0").Click()
	if len(committed) != 1 || committed[0] != "m1" {
		t.Fatalf("click committed %v, want exactly [m1]", committed)
	}
}

func TestPickerCreateRowIsOptionalAndLast(t *testing.T) {
	var created []string
	f := render.New(t)
	renderPickerForTest(f, PickerProps{
		ID: "create-picker", Label: "Category", Placeholder: "Choose", Searchable: true,
		Options:  []PickerOption{{Value: "food", Label: "Food"}},
		OnCreate: func(query string) { created = append(created, query) },
	})
	f.ByID("create-picker").Click()
	search := f.ByID("create-picker-search")
	search.Input("Travel")
	if !f.ByID("create-picker-create").Exists() {
		t.Fatal("typed query did not show create row")
	}
	if got := f.ByID("create-picker-create").Attr("aria-label"); got != "Create Travel" {
		t.Fatalf("create accessible name %q, want Create Travel", got)
	}
	if got := f.ByID("create-picker-create").Text(); got != "Create 'Travel'" {
		t.Fatalf("create action text %q, want Create 'Travel'", got)
	}
	if f.ByID("create-picker-option-1").Exists() {
		t.Fatal("create action appeared among selectable options")
	}
	f.ByID("create-picker-create").Click()
	if len(created) != 1 || created[0] != "Travel" {
		t.Fatalf("create callback received %v, want exactly [Travel]", created)
	}
}

func TestMultiPickerTogglesSelectedAriaState(t *testing.T) {
	var toggled string
	f := render.New(t)
	renderMultiPickerForTest(f, MultiPickerProps{
		ID: "people-picker", Label: "People", Placeholder: "Choose people",
		Options:  []PickerOption{{Value: "m1", Label: "Morgan"}, {Value: "m2", Label: "Taylor"}},
		Selected: []string{"m1"}, OnToggle: func(value string) { toggled = value },
	})
	f.ByID("people-picker").Click()
	if got := f.ByID("people-picker-option-0").Attr("aria-selected"); got != "true" {
		t.Fatalf("selected option aria-selected = %q, want true", got)
	}
	if got := f.ByID("people-picker-option-1").Attr("aria-selected"); got != "false" {
		t.Fatalf("unselected option aria-selected = %q, want false", got)
	}
	f.ByID("people-picker-option-1").Click()
	if toggled != "m2" {
		t.Fatalf("toggle callback value = %q, want m2", toggled)
	}
}

func TestMultiPickerStaysOpenAcrossPicks(t *testing.T) {
	var toggled []string
	f := render.New(t)
	renderMultiPickerForTest(f, MultiPickerProps{
		ID: "stay-picker", Label: "Stay", Placeholder: "Choose",
		Options:  []PickerOption{{Value: "m1", Label: "Morgan"}, {Value: "m2", Label: "Taylor"}},
		OnToggle: func(value string) { toggled = append(toggled, value) },
	})
	f.ByID("stay-picker").Click()
	f.ByID("stay-picker-option-0").Click()
	f.Flush()
	if !f.ByID("stay-picker-listbox").Exists() {
		t.Fatal("the sheet closed after one pick; a multi-select should stay open")
	}
	f.ByID("stay-picker-option-1").Click()
	f.Flush()
	if want := []string{"m1", "m2"}; len(toggled) != 2 || toggled[0] != want[0] || toggled[1] != want[1] {
		t.Fatalf("toggled = %v, want %v", toggled, want)
	}
}
