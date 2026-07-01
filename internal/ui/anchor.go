// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"

	uic "github.com/monstercameron/GoWebComponents/ui"
)

// AnchorPopover keeps an open `.add-menu` popover inside the viewport by toggling
// the `open-left` / `open-up` classes on it when, at its natural below-right
// position, it would overflow the right or bottom edge of the window. The menu is
// only ~210px wide and its trigger (the `⋯` button) typically sits near the right
// edge of a row, so without this the menu spills off-screen on open.
//
// Pair it with DismissPopover on the same wrapID (the id on the `.add-wrap`
// wrapper). Like DismissPopover it is a custom hook (it calls UseEffect), so invoke
// it unconditionally at a stable render position. It measures one frame after the
// menu is shown (so the element has dimensions) and re-measures on resize; the
// classes are reset before each measure so a menu that no longer overflows snaps
// back to its default side.
func AnchorPopover(isOpen bool, wrapID string) {
	openKey := "closed"
	if isOpen {
		openKey = "open"
	}
	uic.UseEffect(func() func() {
		if !isOpen || wrapID == "" {
			return nil
		}
		win := js.Global()
		doc := win.Get("document")
		if !doc.Truthy() {
			return nil
		}

		const margin = 8.0
		reposition := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			// getElementById (not querySelector("#"+id)): UseId() ids contain colons,
			// which are invalid in a "#id" selector and would throw.
			w := doc.Call("getElementById", wrapID)
			if !w.Truthy() {
				return nil
			}
			menu := w.Call("querySelector", ".add-menu")
			if !menu.Truthy() {
				return nil
			}
			cl := menu.Get("classList")
			// Reset to the natural side first so the rect we measure is the default
			// position, then flip only the edges that actually overflow.
			cl.Call("remove", "open-left")
			cl.Call("remove", "open-up")
			r := menu.Call("getBoundingClientRect")
			vw := win.Get("innerWidth").Float()
			vh := win.Get("innerHeight").Float()
			if r.Get("right").Float() > vw-margin {
				cl.Call("add", "open-left")
			}
			if r.Get("bottom").Float() > vh-margin {
				cl.Call("add", "open-up")
			}
			return nil
		})

		// Measure after layout (next frame) so the just-shown menu has real dimensions.
		raf := win.Call("requestAnimationFrame", reposition)
		resizeCb := js.FuncOf(func(_ js.Value, _ []js.Value) any { reposition.Invoke(); return nil })
		win.Call("addEventListener", "resize", resizeCb)
		// Reposition while scrolling too (capture phase catches inner scrollers), so a
		// menu opened near the fold doesn't drift off-screen as the page moves.
		win.Call("addEventListener", "scroll", resizeCb, map[string]any{"capture": true, "passive": true})

		return func() {
			win.Call("cancelAnimationFrame", raf)
			win.Call("removeEventListener", "resize", resizeCb)
			win.Call("removeEventListener", "scroll", resizeCb, map[string]any{"capture": true})
			reposition.Release()
			resizeCb.Release()
		}
	}, openKey)
}
