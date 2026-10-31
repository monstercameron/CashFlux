// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/GoWebComponents/v7/interop"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// DismissPopover closes a `.add-wrap` / `.add-menu` popover when the reader leaves it, and
// handles its arrow-key navigation. wrapID is the id of the wrapper element that holds the
// trigger and the menu. While isOpen is true:
//
//   - Escape calls onClose and moves focus back to the trigger (the first button in the wrapper);
//   - a press outside the wrapper calls onClose;
//   - ArrowDown, ArrowUp, Home and End move focus among the wrapper's [role=menuitem] entries.
//     They do this only when focus is already inside the wrapper, so arrow keys are never
//     taken from the page while the menu is merely open.
//
// A press inside the wrapper is recorded by the wrapper's own pointerdown listener, which is
// attached only while the menu is open. The document listener runs after it, reads the
// record and closes the menu only for a press that started outside. Targets are never
// compared. Because the wrapper listener attaches after the opening click, that click cannot
// close the menu it opened. The document listeners stay mounted and return early while the
// menu is closed. They listen on the document rather than on a backdrop element, since a
// backdrop would sit under the topbar's sticky header.
//
// DismissPopover is a custom hook: it calls UseLayoutEffect and UseDocumentEvent, so call it
// unconditionally at a stable render position, like any hook.
func DismissPopover(isOpen bool, wrapID string, onClose func()) {
	pressedInside := uic.UseRef(false)
	uic.UseLayoutEffect(func() func() {
		if !isOpen || wrapID == "" {
			return nil
		}
		wrap, found := domElement(wrapID)
		if !found {
			return nil
		}
		subscription, err := wrap.Listen("pointerdown", func(interop.BrowserEvent) {
			pressedInside.Set(true)
		})
		if err != nil {
			return nil
		}
		return func() { subscription.Cancel() }
	}, isOpen, wrapID)

	uic.UseDocumentEvent("pointerdown", func(uic.Event) {
		inside := pressedInside.Get()
		pressedInside.Set(false)
		if isOpen && wrapID != "" && onClose != nil && !inside {
			onClose()
		}
	})

	uic.UseDocumentEvent("keydown", func(e uic.Event) {
		if !isOpen || wrapID == "" {
			return
		}
		switch e.GetKey() {
		case "Escape":
			if onClose != nil {
				onClose()
			}
			trigger := domValue(domNode(wrapID), "querySelector", "button")
			domValue(trigger, "focus")
		case "ArrowDown", "ArrowUp", "Home", "End":
			rovePopoverFocus(e, wrapID)
		}
	})
}

// rovePopoverFocus moves focus among a popover's menu items for the arrow and Home/End keys,
// when focus is already inside the popover.
func rovePopoverFocus(e uic.Event, wrapID string) {
	wrap := domNode(wrapID)
	if !wrap.Truthy() {
		return
	}
	active := domDocument().Get("activeElement")
	if !active.Truthy() || !domValue(wrap, "contains", active).Bool() {
		return
	}
	items := domValue(wrap, "querySelectorAll", `[role="menuitem"]`)
	n := items.Get("length").Int()
	if n == 0 {
		return
	}
	e.PreventDefault()
	cur := -1
	for i := range n {
		if domValue(domValue(items, "item", i), "isSameNode", active).Bool() {
			cur = i
			break
		}
	}
	next := 0
	switch e.GetKey() {
	case "ArrowDown":
		if next = cur + 1; next >= n {
			next = 0
		}
	case "ArrowUp":
		if next = cur - 1; next < 0 {
			next = n - 1
		}
	case "End":
		next = n - 1
	}
	domValue(domValue(items, "item", next), "focus")
}
