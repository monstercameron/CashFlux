// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"syscall/js"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestSegmentedControlPreservesChoiceSemanticsAndOptionalCounts(t *testing.T) {
	picked := ""
	props := SegmentedControlProps{
		Class: "existing-switch", Label: "Date lens", ButtonClass: "choice", ActiveClass: "active", CountClass: "badge", Selected: "all",
		Attrs:    []any{Attr("data-testid", "segments")},
		Options:  []SegmentOption{{Key: "all", Label: "All", TestID: "all"}, {Key: "today", Label: "Today", Count: 0, ShowCount: true, TestID: "today"}},
		OnSelect: func(key string) { picked = key },
	}
	f := render.New(t)
	f.Render(SegmentedControl(props))
	root := controlStripByTestID(f, "segments")
	if root.Attr("class") != "existing-switch" || root.Attr("role") != "group" || root.Attr("aria-label") != "Date lens" {
		t.Fatal("segmented group did not retain its classes and accessible name")
	}
	all, today := controlStripByTestID(f, "all"), controlStripByTestID(f, "today")
	if all.Attr("aria-pressed") != "true" || all.Attr("class") != "choice active" || today.Attr("aria-pressed") != "false" {
		t.Fatal("choice state did not map to pressed state and active class")
	}
	var badge *render.QueryNode
	for _, child := range today.Children() {
		if child.Tag() == "span" && child.Attr("class") == "badge" {
			badge = child
		}
	}
	if all.Text() != "All" || !strings.Contains(today.Text(), "Today") || badge == nil || badge.Text() != "0" {
		t.Fatalf("optional zero count did not preserve label/badge: all=%q today=%q badge=%v", all.Text(), today.Text(), badge)
	}
	today.Click()
	f.Stabilize()
	if picked != "today" {
		t.Fatalf("selected %q, want today", picked)
	}
	props.Tabs, props.Selected = true, "today"
	f.Rerender(SegmentedControl(props))
	today = controlStripByTestID(f, "today")
	if controlStripByTestID(f, "segments").Attr("role") != "tablist" || today.Attr("role") != "tab" || today.Attr("aria-selected") != "true" || today.Attr("aria-pressed") != "" {
		t.Fatal("tabs did not switch to selected semantics")
	}
}

func TestSegmentedControlHandlersStayWithKeysAcrossEmptyAndReorderedOptions(t *testing.T) {
	picked := ""
	props := SegmentedControlProps{OnSelect: func(key string) { picked = key }}
	f := render.New(t)
	f.Render(SegmentedControl(props))
	a, b := SegmentOption{Key: "a", Label: "A", TestID: "a"}, SegmentOption{Key: "b", Label: "B", TestID: "b"}
	props.Options = []SegmentOption{a, b}
	f.Rerender(SegmentedControl(props))
	props.Options = []SegmentOption{b, a}
	f.Rerender(SegmentedControl(props))
	controlStripByTestID(f, "b").Click()
	f.Stabilize()
	if picked != "b" {
		t.Fatalf("reordered B invoked %q", picked)
	}
	props.Options = []SegmentOption{b}
	f.Rerender(SegmentedControl(props))
	controlStripByTestID(f, "b").Click()
	f.Stabilize()
	if picked != "b" {
		t.Fatal("removing the preceding option changed the survivor's handler")
	}
	b.Disabled = true
	props.Options = []SegmentOption{b}
	f.Rerender(SegmentedControl(props))
	picked = ""
	controlStripByTestID(f, "b").Click()
	f.Stabilize()
	if picked != "" {
		t.Fatal("disabled segment invoked its callback")
	}
	props.Options = nil
	f.Rerender(SegmentedControl(props))
	props.Options = []SegmentOption{a}
	f.Rerender(SegmentedControl(props))
	controlStripByTestID(f, "a").Click()
	f.Stabilize()
	if picked != "a" {
		t.Fatal("empty-to-populated segment lost its callback")
	}
}

type segmentedKeyboardProps struct {
	Options []SegmentOption
	Picked  *string
}

func segmentedKeyboardFixture(props segmentedKeyboardProps) uic.Node {
	selected := uic.UseState("a")
	return SegmentedControl(SegmentedControlProps{
		Tabs: true, Label: "Keyboard tabs", Selected: selected.Get(), Options: props.Options,
		OnSelect: func(key string) { *props.Picked = key; selected.Set(key) },
	})
}

func TestSegmentedTabsKeyboardSelectionFocusAndKeys(t *testing.T) {
	previousDocument := js.Global().Get("document")
	requested, focused := "", ""
	focus := js.FuncOf(func(js.Value, []js.Value) any { focused = requested; return nil })
	lookup := js.FuncOf(func(_ js.Value, args []js.Value) any {
		requested = args[0].String()
		element := js.Global().Get("Object").New()
		element.Set("focus", focus)
		return element
	})
	document := js.Global().Get("Object").New()
	document.Set("getElementById", lookup)
	js.Global().Set("document", document)
	t.Cleanup(func() { js.Global().Set("document", previousDocument); focus.Release(); lookup.Release() })
	picked := ""
	a := SegmentOption{Key: "a", Label: "A", TestID: "key-a"}
	disabled := SegmentOption{Key: "disabled", Label: "Disabled", Disabled: true, TestID: "key-disabled"}
	c := SegmentOption{Key: "c", Label: "C", TestID: "key-c"}
	props := segmentedKeyboardProps{Options: []SegmentOption{a, disabled, c}, Picked: &picked}
	f := render.New(t)
	f.Render(uic.CreateElement(segmentedKeyboardFixture, props))
	key := func(from, key, want string) {
		t.Helper()
		controlStripByTestID(f, "key-"+from).Dispatch("onkeydown", render.Event{Key: key})
		f.Stabilize()
		target := controlStripByTestID(f, "key-"+want)
		if picked != want || focused != target.Attr("id") || target.Attr("tabindex") != "0" || target.Attr("aria-selected") != "true" {
			t.Fatalf("%s on %s: selected=%q focus=%q, want selected/focused enabled tab %q", key, from, picked, focused, want)
		}
		for _, other := range []string{"a", "disabled", "c"} {
			if other != want {
				if node := controlStripByTestID(f, "key-"+other); node != nil && node.Attr("tabindex") != "-1" {
					t.Fatalf("non-selected tab %q remained in the tab order", other)
				}
			}
		}
	}
	key("a", "ArrowRight", "c")
	key("c", "ArrowRight", "a")
	key("a", "ArrowLeft", "c")
	key("c", "Home", "a")
	key("a", "End", "c")
	cID := controlStripByTestID(f, "key-c").Attr("id")
	props.Options = []SegmentOption{c, a, disabled}
	f.Rerender(uic.CreateElement(segmentedKeyboardFixture, props))
	if node := controlStripByTestID(f, "key-c"); node.Attr("id") != cID || node.Attr("aria-selected") != "true" || node.Attr("tabindex") != "0" {
		t.Fatal("reordering changed the selected key's identity or tab stop")
	}
	key("c", "ArrowRight", "a")
	props.Options = []SegmentOption{a, c}
	f.Rerender(uic.CreateElement(segmentedKeyboardFixture, props))
	if node := controlStripByTestID(f, "key-a"); node.Attr("aria-selected") != "true" || node.Attr("tabindex") != "0" {
		t.Fatal("removing a preceding option moved selection off its stable key")
	}
}
