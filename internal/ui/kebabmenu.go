// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	sh "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// KebabMenuProps configures a KebabMenu.
type KebabMenuProps struct {
	// ID is the stable element id on the `.add-wrap` wrapper. The menu uses it to find its
	// popover and to tell clicks inside it from clicks outside. Supply a value that's stable across
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
	// Items are the pre-built `.add-item` entries. The caller builds them with its own
	// UseEvent handlers, because a loop in this component would register On* hooks in a
	// variable-length list. KebabMenu supplies only the container. Selecting an item closes
	// the menu, because the click bubbles up to the menu's close handler.
	Items []uic.Node
}

// KebabMenu is the shared "⋯" menu for a row of pre-built items. It is built on the same
// menu state as OverflowMenu, so it opens and closes with the same fade, stays on screen
// with the same flip, dismisses on Escape or an outside press, and stacks above its
// neighbours. The caller builds the Items, with its own handlers. Use OverflowMenu when the
// menu can be described as data.
func KebabMenu(props KebabMenuProps) uic.Node { return uic.CreateElement(kebabMenu, props) }

func kebabMenu(props KebabMenuProps) uic.Node {
	autoID := uic.UseId()
	id := props.ID
	if id == "" {
		id = autoID
	}
	menu := UseMenu(id)
	toggle := uic.UseEvent(sh.Prevent(func() { menu.Toggle() }))
	// An item click bubbles up to the menu, which closes it.
	closeM := uic.UseEvent(sh.Prevent(func() { menu.Set(false) }))

	aria := props.AriaLabel
	if aria == "" {
		aria = "More actions"
	}
	wrapCls := "add-wrap"
	if props.WrapClass != "" {
		wrapCls += " " + props.WrapClass
	}
	// The trigger is the shared ToolButton, the same button the More menu opens from.
	trigger := ToolButton(ToolButtonProps{
		Icon: icon.MoreH, Title: aria, AriaLabel: aria, Open: menu.Open,
		Class: props.ToggleClass, TestID: props.ToggleTestID,
	},
		sh.Attr("aria-haspopup", "menu"), sh.Attr("aria-expanded", boolAttr(menu.Open)),
		sh.OnClick(toggle),
	)

	// The items are pre-built by the caller, so the panel is always mounted and its class
	// carries the open, fading and hidden states.
	menuChildren := []any{sh.ClassStr(menu.PanelClass()), sh.Attr("role", "menu"), sh.OnClick(closeM)}
	for _, it := range props.Items {
		menuChildren = append(menuChildren, it)
	}

	return sh.Div(sh.ClassStr(menu.WrapClass(wrapCls)), sh.Attr("id", id),
		trigger,
		sh.Div(menuChildren...),
	)
}
