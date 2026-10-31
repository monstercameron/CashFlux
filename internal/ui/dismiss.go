// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/GoWebComponents/v7/interop"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// DismissPopover wires WAI-ARIA menu-button dismissal for a popover (the
// `.add-wrap` / `.add-menu` pattern) whose WRAPPER element carries `wrapID`:
//
//   - Escape closes the popover and returns focus to its trigger (the first
//     <button> inside the wrapper);
//   - a pointerdown outside the wrapper closes it;
//   - ArrowDown/ArrowUp/Home/End rove focus among the wrapper's [role=menuitem]
//     entries while focus is inside the popover (WAI-ARIA menu keyboard nav).
//
// Presses are told apart without comparing event targets: while the popover is
// open, the wrapper element records its own pointerdown, and the document-level
// listener, which runs after the wrapper's, closes the popover only when the press
// did not start inside the wrapper. Keyboard roving is gated on
// document.activeElement sitting inside the wrapper, so arrow keys are never
// hijacked while the menu is merely open. The document listener is immune to the
// stacking of the topbar's sticky header, which is why the `.add-backdrop` element
// is not used.
//
// It is a custom hook: it calls UseLayoutEffect and UseDocumentEvent, so invoke it
// unconditionally at a stable render position (like any hook). The wrapper's listener
// is attached only while open. The document listeners stay mounted and return early
// while closed. Self-close on open is avoided because the listener attaches after
// the opening click's pointerdown has already fired.
//
// `isOpen` is the popover's current open state; `wrapID` is the id set on the
// wrapper element; `onClose` dismisses the popover (e.g. open.Set(false)).
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
