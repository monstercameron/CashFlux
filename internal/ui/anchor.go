// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"syscall/js"

	uic "github.com/monstercameron/GoWebComponents/ui"
)

// SmartTipPortal renders an explainer popover for the trigger wrapper (#wrapID) as a
// plain DOM node appended to <body> — a PORTAL. This is the only reliable way to make a
// floating popover both (a) escape any overflow:hidden ancestor (e.g. the summary
// "loader" bar) and (b) escape the tile's own stacking context, so it paints ABOVE the
// sibling tiles below it (which otherwise cover it — z-index can't win across sibling
// stacking contexts). It positions the popover fixed below the trigger, right-aligned,
// flipping above / clamping horizontally to stay on screen, and re-measures on the next
// frame, on resize, and while scrolling. Because the node lives in <body> (no transformed
// ancestor), fixed coordinates are viewport-relative. The node is removed on close.
// Pair with DismissPopover on wrapID so an outside click / Escape clears the open state.
func SmartTipPortal(isOpen bool, wrapID, title, text string) {
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
		body := doc.Get("body")
		if !body.Truthy() {
			return nil
		}
		el := doc.Call("createElement", "div")
		el.Set("className", "smart-tip-pop add-menu")
		el.Call("setAttribute", "role", "tooltip")
		el.Call("setAttribute", "data-testid", "smart-tip-pop")
		tEl := doc.Call("createElement", "div")
		tEl.Set("className", "smart-tip-pop-title")
		tEl.Set("textContent", title)
		pEl := doc.Call("createElement", "p")
		pEl.Set("className", "smart-tip-pop-text")
		pEl.Set("textContent", text)
		el.Call("appendChild", tEl)
		el.Call("appendChild", pEl)
		body.Call("appendChild", el)

		const margin = 8.0
		reposition := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			w := doc.Call("getElementById", wrapID)
			if !w.Truthy() {
				return nil
			}
			tr := w.Call("getBoundingClientRect")
			pr := el.Call("getBoundingClientRect")
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
			st := el.Get("style")
			st.Set("top", fmt.Sprintf("%.0fpx", top))
			st.Set("left", fmt.Sprintf("%.0fpx", left))
			st.Set("right", "auto")
			st.Set("bottom", "auto")
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
			if el.Truthy() {
				if pn := el.Get("parentNode"); pn.Truthy() {
					pn.Call("removeChild", el)
				}
			}
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
