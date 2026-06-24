// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"

	uic "github.com/monstercameron/GoWebComponents/ui"
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
// The outside-click path is a document-level listener rather than relying on the
// `.add-backdrop` element, because that backdrop is `position:fixed` inside the
// topbar's sticky stacking context and so does not paint over page content —
// clicks there fell through without dismissing. A document listener is immune to
// stacking and catches outside presses anywhere.
//
// It is a custom hook: it calls UseEffect, so invoke it unconditionally at a
// stable render position (like any hook). The listeners are registered only while
// open and torn down on close/unmount. Self-close on open is avoided because the
// effect registers AFTER the opening click's pointerdown has already fired.
//
// `isOpen` is the popover's current open state; `wrapID` is the id set on the
// wrapper element; `onClose` dismisses the popover (e.g. open.Set(false)).
func DismissPopover(isOpen bool, wrapID string, onClose func()) {
	openKey := "closed"
	if isOpen {
		openKey = "open"
	}
	uic.UseEffect(func() func() {
		if !isOpen || wrapID == "" || onClose == nil {
			return nil
		}
		doc := js.Global().Get("document")
		// getElementById (NOT querySelector("#"+id)): UseId() ids contain colons
		// (e.g. "gwc:3:1"), which are invalid in a "#id" CSS selector — querySelector
		// would throw a SyntaxError and panic the wasm callback. getElementById takes
		// the raw id string and never throws.
		keyCb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			e := args[0]
			key := e.Get("key").String()
			if key == "Escape" {
				onClose()
				if w := doc.Call("getElementById", wrapID); w.Truthy() {
					if btn := w.Call("querySelector", "button"); btn.Truthy() {
						btn.Call("focus")
					}
				}
				return nil
			}
			// Arrow-key roving focus among [role=menuitem] entries (WAI-ARIA menu
			// pattern): ArrowDown/Up cycle, Home/End jump to ends. Gated on focus
			// already being inside THIS popover's wrapper, so arrow keys are never
			// hijacked globally while the menu is merely open-but-unfocused.
			if key == "ArrowDown" || key == "ArrowUp" || key == "Home" || key == "End" {
				w := doc.Call("getElementById", wrapID)
				if !w.Truthy() {
					return nil
				}
				active := doc.Get("activeElement")
				if !active.Truthy() || !w.Call("contains", active).Bool() {
					return nil
				}
				items := w.Call("querySelectorAll", `[role="menuitem"]`)
				n := items.Get("length").Int()
				if n == 0 {
					return nil
				}
				e.Call("preventDefault")
				cur := -1
				for i := range n {
					if items.Index(i).Equal(active) {
						cur = i
						break
					}
				}
				next := 0
				switch key {
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
				items.Index(next).Call("focus")
			}
			return nil
		})
		downCb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			target := args[0].Get("target")
			w := doc.Call("getElementById", wrapID)
			if w.Truthy() && target.Truthy() && !w.Call("contains", target).Bool() {
				onClose()
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", keyCb)
		doc.Call("addEventListener", "pointerdown", downCb)
		return func() {
			doc.Call("removeEventListener", "keydown", keyCb)
			doc.Call("removeEventListener", "pointerdown", downCb)
			keyCb.Release()
			downCb.Release()
		}
	}, openKey)
}
