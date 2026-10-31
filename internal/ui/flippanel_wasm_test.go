// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/interop"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

// focusedControl returns a fake focused element with the given tag and, when formID is set,
// the id of the form it belongs to. An empty formID means the control is not in a form.
func focusedControl(t *testing.T, tagName, formID string) interop.Value {
	t.Helper()
	control := fakeObject(t)
	if err := control.Set("tagName", tagName); err != nil {
		t.Fatal(err)
	}
	if formID == "" {
		return control
	}
	form := fakeObject(t)
	if err := form.Set("id", formID); err != nil {
		t.Fatal(err)
	}
	if err := control.Set("form", form); err != nil {
		t.Fatal(err)
	}
	return control
}

func TestFlipPanelEnterBelongsToPanel(t *testing.T) {
	const footerForm = "import-modal-form"
	cases := []struct {
		name   string
		active interop.Value
		formID string
		want   bool
	}{
		{"no focused element", interop.Value{}, footerForm, true},
		{"button keeps its own Enter", focusedControl(t, "BUTTON", ""), footerForm, false},
		{"text area keeps its own Enter", focusedControl(t, "TEXTAREA", ""), footerForm, false},
		{"select keeps its own Enter", focusedControl(t, "SELECT", ""), footerForm, false},
		{"field outside any form confirms the panel", focusedControl(t, "INPUT", ""), footerForm, true},
		{"field in the footer form confirms the panel", focusedControl(t, "INPUT", footerForm), footerForm, true},
		{"field in a nested form keeps its submit", focusedControl(t, "INPUT", "draft-row-form"), footerForm, false},
		{"field in a form when the panel has no footer form", focusedControl(t, "INPUT", "draft-row-form"), "", false},
	}
	for _, tc := range cases {
		if got := flipPanelEnterBelongsToPanel(tc.active, tc.formID); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFlipPanelInitialFocusSelectorStaysInsideTheBody(t *testing.T) {
	selector := flipPanelInitialFocusSelector("flip-gwc-3-1")
	if !strings.HasPrefix(selector, "#flip-gwc-3-1 [autofocus]") {
		t.Fatalf("initial focus should prefer an explicit autofocus field, got %q", selector)
	}
	if !strings.Contains(selector, "#flip-gwc-3-1 .set-body input") {
		t.Fatalf("initial focus should fall back to the first body field, got %q", selector)
	}
	if strings.Contains(selector, "set-close") {
		t.Fatalf("initial focus must never default to the header Close control, got %q", selector)
	}
}

// unmountFlipPanel renders nothing in place of the panel, so the overlay's effects clean up.
// The fixture's Cleanup does not unmount it, and an overlay left open stays in the framework's
// process-wide overlay stack, leaking its listeners into later tests.
func unmountFlipPanel(f *render.Fixture) {
	f.Render(Fragment())
}
