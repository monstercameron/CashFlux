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
	// ID is the element id of the `.add-wrap` wrapper. The menu uses it to find its popover and to
	// tell a press inside it from a press outside. Give a value that is stable across renders, such
	// as "task-menu-"+entityID. When it is empty, a per-instance id is used.
	ID string
	// AriaLabel is the accessible name of the ⋯ toggle. It defaults to "More actions".
	AriaLabel string
	// WrapClass adds classes to the `.add-wrap`, such as "row-2nd" to fade the trigger in on row hover.
	WrapClass string
	// ToggleTestID is the data-testid of the ⋯ toggle. Empty adds none.
	ToggleTestID string
	// ToggleClass adds classes to the ⋯ toggle, such as a row's icon-button class, so the toggle
	// matches its neighbours.
	ToggleClass string
	// Items are the entries of the menu, built by the caller with its own event handlers. KebabMenu
	// cannot build them in a loop itself, because each On* option registers hooks, and a loop of
	// variable length would register a variable number of them. Choosing an entry closes the menu,
	// because the click bubbles up to the menu's close handler.
	Items []uic.Node
}

// KebabMenu is the shared "⋯" menu for a row of pre-built items. It uses the same menu state as
// OverflowMenu, so it opens and closes with the same fade, stays on screen with the same flip,
// dismisses on Escape or an outside press, and stacks above its neighbours. Use OverflowMenu
// instead when the entries can be described as data.
func KebabMenu(props KebabMenuProps) uic.Node { return uic.CreateElement(kebabMenu, props) }

func kebabMenu(props KebabMenuProps) uic.Node {
	// UseId runs on every render, so the hook order stays the same whether or not an ID is given.
	autoID := uic.UseId()
	id := props.ID
	if id == "" {
		id = autoID
	}
	menu := UseMenu(id)
	toggle := uic.UseEvent(sh.Prevent(func() { menu.Toggle() }))
	// An item click bubbles up to the panel, which closes the menu.
	closeMenu := uic.UseEvent(sh.Prevent(func() { menu.Set(false) }))

	aria := props.AriaLabel
	if aria == "" {
		aria = "More actions"
	}
	wrapCls := "add-wrap"
	if props.WrapClass != "" {
		wrapCls += " " + props.WrapClass
	}
	// The trigger is the shared ToolButton, so it matches the other tool buttons.
	trigger := ToolButton(ToolButtonProps{
		Icon: icon.MoreH, Title: aria, AriaLabel: aria, Open: menu.Open,
		Class: props.ToggleClass, TestID: props.ToggleTestID,
	},
		sh.Attr("aria-haspopup", "menu"), sh.Attr("aria-expanded", boolAttr(menu.Open)),
		sh.OnClick(toggle),
	)

	// The panel is always mounted, so its class carries the open, fading and hidden states.
	panelChildren := []any{sh.ClassStr(menu.PanelClass()), sh.Attr("role", "menu"), sh.OnClick(closeMenu)}
	for _, item := range props.Items {
		panelChildren = append(panelChildren, item)
	}

	return sh.Div(sh.ClassStr(menu.WrapClass(wrapCls)), sh.Attr("id", id),
		trigger,
		sh.Div(panelChildren...),
	)
}
