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

// DisclosureProps configures a Disclosure.
type DisclosureProps struct {
	// ID is the element id of the body region. The toggle's aria-controls points at it, so
	// give a value that is stable across renders. When it is empty, a per-instance id is used.
	ID string
	// Summary is the content of the toggle button, such as a label or a label with a count.
	Summary uic.Node
	// Body is the region the toggle shows and hides. It stays mounted while it fades out.
	Body uic.Node
	// Open is the initial state. After the first render the Disclosure owns its state, and a
	// later change to this field does not move it.
	Open bool
	// TestID is the data-testid of the toggle button. Empty adds none. The body region gets
	// the same value with "-body" appended.
	TestID string
	// Class adds classes to the root element.
	Class string
	// OnToggle is called after each toggle with the new open state, so the caller can persist
	// it. The Disclosure's own state stays the source of truth for rendering. It may be nil.
	OnToggle func(open bool)
}

// Disclosure renders a real button that shows and hides a body region. The button reports its
// state through aria-expanded and names the region through aria-controls. The body fades in and
// out through UsePanelFade and is removed from the DOM once the fade ends. Each Disclosure owns
// its state and click hook, so it is safe inside a variable-length loop.
func Disclosure(props DisclosureProps) uic.Node { return uic.CreateElement(disclosure, props) }

func disclosure(props DisclosureProps) uic.Node {
	// UseId runs on every render, so the hook order stays the same whether or not an ID is given.
	autoID := uic.UseId()
	id := props.ID
	if id == "" {
		id = autoID
	}
	open := uic.UseState(props.Open)
	mounted := UsePanelFade(open.Get())
	onToggle := props.OnToggle
	toggle := uic.UseEvent(Prevent(func() {
		next := !open.Get()
		open.Set(next)
		if onToggle != nil {
			onToggle(next)
		}
	}))

	rootCls := "disclosure"
	if props.Class != "" {
		rootCls += " " + props.Class
	}
	btnArgs := []any{
		ClassStr("disclosure-btn"),
		Type("button"),
		Attr("aria-expanded", boolAttr(open.Get())),
		Attr("aria-controls", id),
		OnClick(toggle),
	}
	if props.TestID != "" {
		btnArgs = append(btnArgs, Attr("data-testid", props.TestID))
	}
	btnArgs = append(btnArgs, Span(css.Class("disclosure-summary"), props.Summary),
		Icon(icon.ChevronDown, css.Class("disclosure-chevron", tw.W4, tw.H4)))

	rootArgs := []any{ClassStr(rootCls), Button(btnArgs...)}
	if mounted {
		bodyArgs := []any{ClassStr(FadeClass("disclosure-body", open.Get())), Attr("id", id)}
		if props.TestID != "" {
			bodyArgs = append(bodyArgs, Attr("data-testid", props.TestID+"-body"))
		}
		if props.Body != nil {
			bodyArgs = append(bodyArgs, props.Body)
		}
		rootArgs = append(rootArgs, Div(bodyArgs...))
	}
	return Div(rootArgs...)
}
