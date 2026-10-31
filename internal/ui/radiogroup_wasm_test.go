// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

// TestRadioGroupComponent checks the radiogroup contract a screen relies on: the group
// and its radios carry the right roles, only the chosen option reports aria-checked, and
// the chosen option holds the single tab stop.
func TestRadioGroupComponent(t *testing.T) {
	f := render.New(t)
	options := []RadioOption{
		{Value: "", Label: "All"},
		{Value: "assets", Label: "Assets"},
		{Value: "liabilities", Label: "Liabilities", Disabled: true},
	}
	f.Render(RadioGroup(RadioGroupProps{Label: "Account class", Value: "assets", Options: options}))
	if f.ByRole("radiogroup", "Account class") == nil {
		t.Fatal("RadioGroup did not render a role=radiogroup element")
	}
	checked := f.ByRole("radio", "Assets")
	if checked == nil || checked.Attr("aria-checked") != "true" || checked.Attr("tabindex") != "0" {
		t.Fatal("the chosen option must report aria-checked=true and hold the tab stop")
	}
	if all := f.ByRole("radio", "All"); all == nil || all.Attr("aria-checked") != "false" || all.Attr("tabindex") != "-1" {
		t.Fatal("an unchosen option must report aria-checked=false and stay out of the tab order")
	}
}
