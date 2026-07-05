// SPDX-License-Identifier: MIT

//go:build js && wasm

// This file uses a qualified import for html/shorthand (alias "sh") rather than
// the usual dot-import because our exported symbols — SelectOption, SelectInput —
// otherwise clash with shorthand's Option() and Select() element functions.

package ui

import (
	sh "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// SelectOption and OptionsFrom are defined in select_pure.go (no build tag) so
// they can be unit-tested on native Go without a browser/wasm runtime.

// ---------------------------------------------------------------------------
// SelectInput
// ---------------------------------------------------------------------------

// SelectInputProps configures a SelectInput control.
type SelectInputProps struct {
	// Options is the list of choices. Build with OptionsFrom or inline literals.
	Options []SelectOption
	// Selected is the currently-selected option value.
	Selected string
	// OnChange is called with the new value when the user picks an option.
	// May be nil (the select renders but does nothing on change).
	OnChange func(value string)
	// AriaLabel is the accessible name. Required when the select has no associated
	// <label> element (passed as aria-label). WCAG 1.3.1 / 4.1.2.
	AriaLabel string
	// Class is extra CSS classes appended to the base "field" class.
	Class string
	// TestID is an optional data-testid attribute for e2e selectors.
	TestID string
}

// SelectInput renders a native <select> element from a typed option list. It
// consolidates the 103× hand-rolled Option-loop pattern found across screens:
// callers supply SelectInputProps (built with OptionsFrom or inline literals)
// instead of repeating the SelectedIf loop themselves.
//
// SelectInput is its own component (wrapped in uic.CreateElement) so its
// OnChange hook stays at a stable render position — safe to call inside rows
// that are themselves inside variable-length lists.
func SelectInput(props SelectInputProps) uic.Node {
	return uic.CreateElement(selectInputWidget, props)
}

func selectInputWidget(props SelectInputProps) uic.Node {
	cls := "field"
	if props.Class != "" {
		cls += " " + props.Class
	}
	onChange := props.OnChange

	args := []any{sh.ClassStr(cls)}
	if props.AriaLabel != "" {
		args = append(args, sh.Attr("aria-label", props.AriaLabel))
	}
	if props.TestID != "" {
		args = append(args, sh.Attr("data-testid", props.TestID))
	}
	args = append(args, sh.OnChange(func(e uic.Event) {
		if onChange != nil {
			onChange(e.GetValue())
		}
	}))

	for _, o := range props.Options {
		args = append(args, sh.Option(sh.Value(o.Value), sh.SelectedIf(props.Selected == o.Value), o.Label))
	}
	return sh.Select(args...)
}
