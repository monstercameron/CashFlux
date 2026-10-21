// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"syscall/js"

	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// AccessibleOverlay records an anchor but leaves placement to its caller.
// Position our portaled popovers against that anchor, flipping above near the
// bottom edge and keeping them in view as an inner scroller moves.
func useOverlayPopoverAnchor(open bool, anchorID, surfaceID string) {
	uic.UseEffect(func() func() {
		if !open || anchorID == "" {
			return nil
		}
		win := js.Global()
		doc := win.Get("document")
		if !doc.Truthy() || win.Get("requestAnimationFrame").Type() != js.TypeFunction {
			return nil // The WASM test renderer has no browser document.
		}
		const margin = 8.0
		place := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			anchor := doc.Call("getElementById", anchorID)
			surface := doc.Call("getElementById", surfaceID)
			if !anchor.Truthy() || !surface.Truthy() {
				return nil
			}
			vw, vh := win.Get("innerWidth").Float(), win.Get("innerHeight").Float()
			style := surface.Get("style")
			style.Set("maxHeight", fmt.Sprintf("%.0fpx", vh-2*margin))
			a := anchor.Call("getBoundingClientRect")
			s := surface.Call("getBoundingClientRect")
			width, height := s.Get("width").Float(), s.Get("height").Float()
			left := a.Get("left").Float()
			if left+width > vw-margin {
				left = vw - width - margin
			}
			if left < margin {
				left = margin
			}
			top := a.Get("bottom").Float() + margin
			// Leave room for the mobile bottom navigation and floating actions.
			bottomEdge := vh - margin
			if vw <= 600 {
				bottomEdge = vh - 72
			}
			if top+height > bottomEdge {
				top = a.Get("top").Float() - height - margin
			}
			if top < margin {
				top = margin
			}
			style.Set("left", fmt.Sprintf("%.0fpx", left))
			style.Set("top", fmt.Sprintf("%.0fpx", top))
			surface.Call("setAttribute", "data-overlay-placed", "true")
			return nil
		})
		raf := win.Call("requestAnimationFrame", place)
		onMove := js.FuncOf(func(_ js.Value, _ []js.Value) any { place.Invoke(); return nil })
		win.Call("addEventListener", "resize", onMove)
		win.Call("addEventListener", "scroll", onMove, map[string]any{"capture": true, "passive": true})
		return func() {
			win.Call("cancelAnimationFrame", raf)
			win.Call("removeEventListener", "resize", onMove)
			win.Call("removeEventListener", "scroll", onMove, map[string]any{"capture": true})
			place.Release()
			onMove.Release()
		}
	}, open, anchorID, surfaceID)
}
