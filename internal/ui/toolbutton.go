// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// ToolButtonProps configures a ToolButton.
type ToolButtonProps struct {
	// Label is the visible text. Leave it empty for an icon-only button, and name the button
	// with Title or AriaLabel.
	Label string
	// Icon is an optional leading glyph.
	Icon icon.Name
	// Badge is an optional count shown in an accent badge after the label.
	Badge string
	// Variant picks a tint: "" for the standard tool button, "primary" for the main action of a
	// toolbar, "stale" for an action that needs attention.
	Variant string
	// Active marks a button whose filter or choice is applied.
	Active bool
	// Open marks a button whose menu or panel is showing.
	Open bool
	// Title is the tooltip. It defaults to Label.
	Title string
	// AriaLabel is the accessible name. It defaults to Label, then Title.
	AriaLabel string
	// Class adds classes for a page that needs one more. It does not replace the base look.
	Class string
	// TestID names the button for tests. Empty adds no test id.
	TestID string
	// Href makes the control a link to Href, with the same look. Empty keeps it a button.
	Href string
}

// ToolButton is the one button for page actions. The Filters button is the reference: a
// `btn btn-tool` with an optional leading glyph, a label, and an accent badge for a count.
// Every toolbar and row action on a page is built with it, so they share one size, radius,
// border and state look. Callers add their handlers and ARIA attributes as extra options,
// so the hooks stay at the caller's render position.
func ToolButton(props ToolButtonProps, extra ...any) uic.Node {
	class := "btn btn-tool"
	switch props.Variant {
	case "primary":
		class += " btn-primary"
	case "stale":
		class += " bt-stale"
	}
	if props.Label == "" && props.Badge == "" {
		class += " is-icon-only"
	}
	if props.Active {
		class += " active"
	}
	if props.Open {
		class += " is-open"
	}
	if props.Class != "" {
		class += " " + props.Class
	}
	tooltip := props.Title
	if tooltip == "" {
		tooltip = props.Label
	}
	name := props.AriaLabel
	if name == "" {
		name = props.Label
	}
	if name == "" {
		name = tooltip
	}
	args := []any{css.Class(class), Attr("aria-label", name)}
	if props.Href == "" {
		args = append(args, Type("button"))
	}
	if tooltip != "" {
		args = append(args, Attr("title", tooltip))
	}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	if props.Icon != "" {
		args = append(args, Icon(props.Icon, css.Class(tw.ShrinkO, tw.W4, tw.H4)))
	}
	if props.Label != "" {
		args = append(args, Span(props.Label))
	}
	if props.Badge != "" {
		args = append(args, Span(css.Class("filter-badge"), Attr("aria-hidden", "true"), props.Badge))
	}
	args = append(args, extra...)
	if props.Href != "" {
		args = append(args, Href(props.Href))
		return A(args...)
	}
	return Button(args...)
}
