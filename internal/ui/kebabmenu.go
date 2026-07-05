// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v4/css"
	sh "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// KebabMenuProps configures a KebabMenu.
type KebabMenuProps struct {
	// ID is the stable element id on the `.add-wrap` wrapper — AnchorPopover and
	// DismissPopover use it to find the popover. Supply a value that's stable across
	// renders (e.g. "task-menu-"+entityID); when empty a per-instance id is used.
	ID string
	// AriaLabel labels the ⋯ toggle button (default "More actions").
	AriaLabel string
	// WrapClass adds extra classes to the `.add-wrap` (e.g. "row-2nd" to fade the
	// trigger in on row hover).
	WrapClass string
	// ToggleTestID sets an optional data-testid on the ⋯ toggle button.
	ToggleTestID string
	// ToggleClass overrides the ⋯ toggle button's class (default "btn"); e.g. pass a
	// row's icon-button class so the trigger matches its neighbours.
	ToggleClass string
	// Items are the pre-built `.add-item` menu entries. The CALLER builds them (with
	// its own UseEvent handlers) so no On* hook is registered inside a loop here —
	// KebabMenu only supplies the container chrome. Selecting any item closes the menu
	// (the click bubbles to the menu's own close handler).
	Items []uic.Node
}

// KebabMenu is the shared "⋯" overflow menu: a trigger button plus a popover that is
// VIEWPORT-AWARE — AnchorPopover flips it left/up when it would spill past an edge, and
// DismissPopover closes it on Escape / outside-pointerdown (with menu keyboard roving).
// It replaces the hand-rolled `.add-wrap`/`.add-menu` blocks the entity rows each copied,
// and guarantees the menu never overflows its container (the bug that scrolled the to-do
// page sideways). Pass pre-built `.add-item` nodes as Items.
func KebabMenu(props KebabMenuProps) uic.Node { return uic.CreateElement(kebabMenu, props) }

func kebabMenu(props KebabMenuProps) uic.Node {
	open := uic.UseState(false)
	autoID := uic.UseId()
	id := props.ID
	if id == "" {
		id = autoID
	}
	toggle := uic.UseEvent(sh.Prevent(func() { open.Set(!open.Get()) }))
	closeM := uic.UseEvent(sh.Prevent(func() { open.Set(false) }))
	// Viewport-aware placement + dismissal (declared unconditionally — they're hooks).
	DismissPopover(open.Get(), id, func() { open.Set(false) })
	AnchorPopover(open.Get(), id)

	hidden := ""
	if !open.Get() {
		hidden = " hidden-menu"
	}
	aria := props.AriaLabel
	if aria == "" {
		aria = "More actions"
	}
	wrapCls := "add-wrap"
	if props.WrapClass != "" {
		wrapCls += " " + props.WrapClass
	}
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}

	toggleClass := props.ToggleClass
	if toggleClass == "" {
		toggleClass = "btn"
	}
	toggleArgs := []any{
		css.Class(toggleClass), sh.Type("button"),
		sh.Attr("title", aria), sh.Attr("aria-label", aria),
		sh.Attr("aria-haspopup", "menu"), sh.Attr("aria-expanded", expanded),
		sh.OnClick(toggle),
	}
	if props.ToggleTestID != "" {
		toggleArgs = append(toggleArgs, sh.Attr("data-testid", props.ToggleTestID))
	}
	toggleArgs = append(toggleArgs, Icon(icon.MoreH, css.Class(tw.W4, tw.H4)))

	// Menu children: role + a close-on-click (an item click bubbles up and dismisses),
	// then the caller's pre-built items.
	menuChildren := []any{sh.ClassStr("add-menu" + hidden), sh.Attr("role", "menu"), sh.OnClick(closeM)}
	for _, it := range props.Items {
		menuChildren = append(menuChildren, it)
	}

	return sh.Div(sh.ClassStr(wrapCls), sh.Attr("id", id),
		sh.Button(toggleArgs...),
		sh.Div(sh.ClassStr("add-backdrop"+hidden), sh.OnClick(closeM)),
		sh.Div(menuChildren...),
	)
}
