// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import uic "github.com/monstercameron/GoWebComponents/v7/ui"

// MenuControl is what a popover menu renders from: whether it is open, whether it is still
// on screen while it fades out, the flip classes the anchor picked for it, and the means to
// open, close and toggle it.
type MenuControl struct {
	Open      bool
	Shown     bool
	Placement string
	Toggle    func()
	Set       func(bool)
}

// useMenu owns a popover menu's state. It keeps the menu mounted for its fade-out, places it
// beside its trigger, and closes it on Escape or on a press outside it. Every popover menu in
// the app is built on it, so they all open, close, place and stack the same way. Call it
// unconditionally at a stable render position, with the id of the menu's wrapper.
func UseMenu(id string) MenuControl {
	open := uic.UseState(false)
	mounted := usePanelFade(open.Get())
	placement := uic.UseState("")
	dismissOverflowMenu(open.Get(), id, func() { open.Set(false) })
	anchorOverflowMenu(open.Get(), id, placement.Set)
	return MenuControl{
		Open:      open.Get(),
		Shown:     open.Get() || mounted,
		Placement: placement.Get(),
		Toggle:    func() { open.Set(!open.Get()) },
		Set:       open.Set,
	}
}

// panelClass is the class of a menu's panel: the shared fade style and placement, plus
// is-leaving while it fades out, or hidden-menu once it is off screen.
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

// wrapClass adds the state a table row or tile reads to rise above its neighbours while the
// menu is on screen, so the menu is never painted under the next row.
func (m MenuControl) WrapClass(base string) string {
	if m.Shown {
		return base + " overflow-menu-open"
	}
	return base
}
