// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import uic "github.com/monstercameron/GoWebComponents/v7/ui"

// MenuControl is what a popover menu renders from: whether it is open, whether it is on screen
// (which includes the fade-out after it closes), the flip classes the anchor picked for it, and
// the means to open, close and toggle it.
type MenuControl struct {
	Open      bool
	Shown     bool
	Placement string
	Toggle    func()
	Set       func(bool)
}

// UseMenu owns the state of a popover menu. The menu stays mounted through its fade-out, is
// placed beside its trigger, and closes on Escape or on a press outside it. The overflow and
// kebab menus both use it, so they open, close, place and stack the same way. Call it
// unconditionally at a stable render position, with the id of the menu's wrapper.
func UseMenu(id string) MenuControl {
	open := uic.UseState(false)
	shown := UsePanelFade(open.Get())
	placement := uic.UseState("")
	DismissPopover(open.Get(), id, func() { open.Set(false) })
	anchorOverflowMenu(open.Get(), id, placement.Set)
	return MenuControl{
		Open:      open.Get(),
		Shown:     shown,
		Placement: placement.Get(),
		Toggle:    func() { open.Set(!open.Get()) },
		Set:       open.Set,
	}
}

// PanelClass returns the class list for a menu's panel: the shared fade style and the placement,
// plus is-leaving while the panel fades out, or hidden-menu once it is off screen.
func (m MenuControl) PanelClass() string {
	class := "add-menu fade-panel" + m.Placement
	switch {
	case m.Open:
	case m.Shown:
		class += " is-leaving"
	default:
		class += " hidden-menu"
	}
	return class
}

// WrapClass adds overflow-menu-open to base while the menu is on screen. That class lifts the
// row or tile above its neighbours, so the menu is not painted under the next row.
func (m MenuControl) WrapClass(base string) string {
	if m.Shown {
		return base + " overflow-menu-open"
	}
	return base
}
