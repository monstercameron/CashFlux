// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// deepLinkSettleMS is how long to wait after a deep-link navigation before looking
// for the target element — enough for the destination route to mount its rows.
const deepLinkSettleMS = 160

// deepLinkFlashMS is how long the highlight pulse stays on the target before it's
// removed. Kept in sync with the .deeplink-flash animation duration in CSS.
const deepLinkFlashMS = 1700

// DeepLinkFocusHost is mounted once at the shell root. It watches the deep-link
// focus atom (set by a notification click that resolved a specific account/budget)
// and, after the destination page settles, scrolls that element into view and
// pulses it so the user sees exactly which item the alert was about. It renders
// nothing. Motion is gated in CSS (prefers-reduced-motion tones the pulse down to
// a static ring), so the Go side stays a thin scroll-and-tag.
func DeepLinkFocusHost() uic.Node {
	focus := uistate.UseDeepLinkFocus()
	sel := focus.Get()

	uic.UseEffect(func() func() {
		if sel == "" {
			return nil
		}
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return nil
		}
		// Wait for the target route to render its rows, then locate + flash the element.
		fired := false
		var settle js.Func
		settle = js.FuncOf(func(js.Value, []js.Value) any {
			fired = true
			settle.Release()
			el := doc.Call("querySelector", sel)
			if el.Truthy() {
				el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "center"})
				cl := el.Get("classList")
				cl.Call("add", "deeplink-flash")
				var clear js.Func
				clear = js.FuncOf(func(js.Value, []js.Value) any {
					clear.Release()
					if el.Truthy() {
						el.Get("classList").Call("remove", "deeplink-flash")
					}
					return nil
				})
				js.Global().Call("setTimeout", clear, deepLinkFlashMS)
			}
			// Consume the request so a later navigation to the same target re-fires.
			focus.Set("")
			return nil
		})
		id := js.Global().Call("setTimeout", settle, deepLinkSettleMS)
		return func() {
			js.Global().Call("clearTimeout", id)
			if !fired {
				settle.Release()
			}
		}
	}, sel)

	return Fragment()
}
