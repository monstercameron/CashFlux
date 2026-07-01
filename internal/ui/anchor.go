// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"syscall/js"

	uic "github.com/monstercameron/GoWebComponents/ui"
)

// AnchorFixedPopover positions an open `.smart-tip-pop` explainer as a FIXED-position
// overlay next to its trigger wrapper (#wrapID). Unlike AnchorPopover (absolute, which is
// clipped by any overflow:hidden ancestor and stacks inside that ancestor's context), a
// fixed popover escapes clipping containers (e.g. the summary "loader" bar) and sits in
// the viewport's own stacking context, so its z-index is respected globally. It opens
// below the trigger, right-aligned, and flips above / clamps horizontally to stay on
// screen; it re-measures on the next frame, on resize, and while scrolling. Pair with
// DismissPopover on the same wrapID.
func AnchorFixedPopover(isOpen bool, wrapID string) {
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
			w := doc.Call("getElementById", wrapID)
			if !w.Truthy() {
				return nil
			}
			pop := w.Call("querySelector", ".smart-tip-pop")
			if !pop.Truthy() {
				return nil
			}
			tr := w.Call("getBoundingClientRect")
			style := pop.Get("style")
			style.Set("position", "fixed")
			style.Set("right", "auto")
			style.Set("bottom", "auto")
			pr := pop.Call("getBoundingClientRect")
			vw := win.Get("innerWidth").Float()
			vh := win.Get("innerHeight").Float()
			pw := pr.Get("width").Float()
			ph := pr.Get("height").Float()
			// Vertical: below the trigger; flip above if it would overflow the bottom.
			top := tr.Get("bottom").Float() + 6
			if top+ph > vh-margin {
				top = tr.Get("top").Float() - ph - 6
			}
			if top < margin {
				top = margin
			}
			// Horizontal: right-align to the trigger, then clamp inside the viewport.
			left := tr.Get("right").Float() - pw
			if left+pw > vw-margin {
				left = vw - pw - margin
			}
			if left < margin {
				left = margin
			}
			// A position:fixed element is laid out relative to its nearest ancestor that
			// establishes a containing block (any transform / filter / perspective — which
			// our animated cards/tiles carry), NOT the viewport. Subtract that ancestor's
			// offset so the viewport coords we computed actually land on screen.
			cbLeft, cbTop := 0.0, 0.0
			anc := pop.Get("parentElement")
			for anc.Truthy() {
				cs := win.Call("getComputedStyle", anc)
				tf := cs.Call("getPropertyValue", "transform").String()
				pe := cs.Call("getPropertyValue", "perspective").String()
				fl := cs.Call("getPropertyValue", "filter").String()
				if (tf != "" && tf != "none") || (pe != "" && pe != "none") || (fl != "" && fl != "none") {
					ar := anc.Call("getBoundingClientRect")
					cbLeft, cbTop = ar.Get("left").Float(), ar.Get("top").Float()
					break
				}
				anc = anc.Get("parentElement")
			}
			style.Set("top", fmt.Sprintf("%.0fpx", top-cbTop))
			style.Set("left", fmt.Sprintf("%.0fpx", left-cbLeft))
			return nil
		})
		raf := win.Call("requestAnimationFrame", reposition)
		resizeCb := js.FuncOf(func(_ js.Value, _ []js.Value) any { reposition.Invoke(); return nil })
		win.Call("addEventListener", "resize", resizeCb)
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
