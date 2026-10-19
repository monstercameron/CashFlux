// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTxnBulkPickerArrowKeysDoNotApplyAndEnterAppliesOnce(t *testing.T) {
	var applied []string
	f := render.New(t)
	f.Render(ui.CreateElement(txnBulkPicker, txnBulkPickerProps{
		ID: "bulk-category-picker", Label: "Category", Placeholder: "Choose a category", SearchLabel: "Search categories",
		Options:  []txnBulkPickerOption{{Value: "", Label: "Category"}, {Value: "food", Label: "Food"}, {Value: "rent", Label: "Rent"}},
		OnChoose: func(value string) { applied = append(applied, value) },
	}))

	trigger := f.ByID("bulk-category-picker")
	if trigger == nil {
		t.Fatal("category picker trigger not rendered")
	}
	trigger.Dispatch("onkeydown", render.Event{Key: "ArrowDown"})
	search := f.ByID("bulk-category-picker-search")
	if search == nil {
		t.Fatal("opening the picker did not show its search input")
	}
	if got := search.Attr("aria-controls"); got != "bulk-category-picker-listbox" {
		t.Fatalf("search controls %q, want the category listbox", got)
	}
	if got := search.Attr("aria-activedescendant"); got != "bulk-category-picker-option-0" {
		t.Fatalf("initial active descendant %q, want first real category option", got)
	}
	if f.ByID("bulk-category-picker-option-0").Text() != "Food" {
		t.Fatal("placeholder rendered as an option; first option should be Food")
	}
	search.Dispatch("onkeydown", render.Event{Key: "ArrowDown"})
	if got := f.ByID("bulk-category-picker-option-1").Attr("aria-selected"); got != "true" {
		t.Fatalf("ArrowDown did not activate the second category: aria-selected=%q", got)
	}
	if len(applied) != 0 {
		t.Fatalf("arrow navigation applied %v; want no changes before confirmation", applied)
	}
	search.Dispatch("onkeydown", render.Event{Key: "Enter"})
	if len(applied) != 1 || applied[0] != "rent" {
		t.Fatalf("Enter applied %v, want exactly [rent]", applied)
	}
	if f.ByID("bulk-category-picker-search") != nil {
		t.Fatal("picker remained open after applying a choice")
	}
}

func TestTxnBulkPickerEscapeClosesWithoutApplying(t *testing.T) {
	called := 0
	f := render.New(t)
	previousDocument := js.Global().Get("document")
	var focused string
	searchFocus := js.FuncOf(func(js.Value, []js.Value) any { focused = "search"; return nil })
	triggerFocus := js.FuncOf(func(js.Value, []js.Value) any { focused = "trigger"; return nil })
	getElementByID := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return js.Null()
		}
		element := js.Global().Get("Object").New()
		if args[0].String() == "bulk-member-picker-search" {
			element.Set("focus", searchFocus)
		} else if args[0].String() == "bulk-member-picker" {
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
	f.Render(ui.CreateElement(txnBulkPicker, txnBulkPickerProps{
		ID: "bulk-member-picker", Label: "Person", Placeholder: "Choose a person", SearchLabel: "Search people",
		Options: []txnBulkPickerOption{{Value: "", Label: "Person"}, {Value: "m1", Label: "Morgan"}, {Value: "m2", Label: "Taylor"}}, OnChoose: func(string) { called++ },
	}))
	f.ByID("bulk-member-picker").Click()
	if focused != "search" {
		t.Fatalf("opening picker focused %q, want search input", focused)
	}
	search := f.ByID("bulk-member-picker-search")
	if got := search.Attr("aria-controls"); got != "bulk-member-picker-listbox" {
		t.Fatalf("person search controls %q, want the person listbox", got)
	}
	if got := f.ByID("bulk-member-picker-option-0").Text(); got != "Morgan" {
		t.Fatalf("first person option is %q, want Morgan (no placeholder option)", got)
	}
	search.Dispatch("onkeydown", render.Event{Key: "ArrowDown"})
	if got := f.ByID("bulk-member-picker-option-1").Attr("aria-selected"); got != "true" {
		t.Fatalf("ArrowDown did not activate the second person: aria-selected=%q", got)
	}
	search.Dispatch("onkeydown", render.Event{Key: "Escape"})
	if called != 0 {
		t.Fatalf("Escape applied a value %d times", called)
	}
	if f.ByID("bulk-member-picker-search") != nil {
		t.Fatal("Escape did not close the picker")
	}
	if focused != "trigger" {
		t.Fatalf("Escape returned focus to %q, want picker button", focused)
	}
}

func TestTxnBulkPersonPickerOmitsPlaceholderOption(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(txnBulkPicker, txnBulkPickerProps{
		ID: "bulk-member-picker", Label: "Person", Placeholder: "Person", SearchLabel: "Search people",
		Options: []txnBulkPickerOption{{Value: "", Label: "Person"}, {Value: "m1", Label: "Morgan"}},
	}))
	f.ByID("bulk-member-picker").Click()
	if got := f.ByID("bulk-member-picker-option-0").Text(); got != "Morgan" {
		t.Fatalf("first person option is %q, want Morgan (no placeholder option)", got)
	}
	if f.ByID("bulk-member-picker-option-1") != nil {
		t.Fatal("person picker rendered a placeholder option")
	}
}

func TestTxnBulkPickerClickAppliesTheChosenOptionOnce(t *testing.T) {
	var applied []string
	f := render.New(t)
	f.Render(ui.CreateElement(txnBulkPicker, txnBulkPickerProps{
		ID: "bulk-member-picker", Label: "Person", Placeholder: "Choose a person", SearchLabel: "Search people",
		Options:  []txnBulkPickerOption{{Value: "m1", Label: "Morgan"}},
		OnChoose: func(value string) { applied = append(applied, value) },
	}))
	f.ByID("bulk-member-picker").Click()
	option := f.ByID("bulk-member-picker-option-0")
	if option == nil {
		t.Fatal("opening the picker did not render its option")
	}
	option.Click()
	if len(applied) != 1 || applied[0] != "m1" {
		t.Fatalf("click applied %v, want exactly [m1]", applied)
	}
}
