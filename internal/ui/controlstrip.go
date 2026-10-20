// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// ControlStripProps configures a horizontal group of related controls and actions.
type ControlStripProps struct {
	// Label, when set, gives the strip an accessible group name.
	Label string
	// Class adds classes to the outer filter-strip container.
	Class string
	// ControlsClass and ActionsClass override the corresponding inner group classes.
	// Empty values use filter-strip-controls for both groups.
	ControlsClass string
	ActionsClass  string
	// Controls and Actions are prebuilt nodes rendered in separate groups.
	Controls []uic.Node
	Actions  []uic.Node
	// Attrs are extra attributes for the outer container.
	Attrs []any
}

// ControlStrip renders caller-owned controls and actions in the shared filter-strip
// layout. It owns no hooks; interactive child components retain their own handlers.
func ControlStrip(props ControlStripProps) uic.Node {
	className := "filter-strip"
	if props.Class != "" {
		className += " " + props.Class
	}
	args := []any{ClassStr(className)}
	args = append(args, props.Attrs...)
	if props.Label != "" {
		args = append(args, Attr("role", "group"), Attr("aria-label", props.Label))
	}
	if len(props.Controls) > 0 {
		controlsClass := props.ControlsClass
		if controlsClass == "" {
			controlsClass = "filter-strip-controls"
		}
		children := make([]any, 0, len(props.Controls)+1)
		children = append(children, ClassStr(controlsClass))
		for _, node := range props.Controls {
			children = append(children, node)
		}
		args = append(args, Div(children...))
	}
	if len(props.Actions) > 0 {
		actionsClass := props.ActionsClass
		if actionsClass == "" {
			actionsClass = "filter-strip-controls"
		}
		children := make([]any, 0, len(props.Actions)+1)
		children = append(children, ClassStr(actionsClass))
		for _, node := range props.Actions {
			children = append(children, node)
		}
		args = append(args, Div(children...))
	}
	return Div(args...)
}
