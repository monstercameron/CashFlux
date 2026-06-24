//go:build js && wasm

package app

import "syscall/js"

// triggerPageEnter replays the W-9 page-enter animation on the #cf-page-view
// element. It removes the .page-enter class so the browser stops any running
// animation, waits one requestAnimationFrame for the style to flush, then
// re-adds the class to restart the keyframe from the beginning. This is the
// canonical browser pattern for restarting a CSS animation without cloning the
// DOM node.
//
// After the animation class is toggled it also calls
// window.cashfluxWonder.observe() (W-21) so the IntersectionObserver picks up
// newly rendered .card elements on the incoming page.
//
// The function is a no-op when the element is not found (e.g. during tests).
// All gating (data-wonder="off", prefers-reduced-motion) is handled in CSS and
// in wonder.js so no Go-side feature-detection is needed here.
func triggerPageEnter() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	el := doc.Call("getElementById", "cf-page-view")
	if el.IsNull() || el.IsUndefined() {
		return
	}
	classList := el.Get("classList")
	classList.Call("remove", "page-enter")
	// Double-rAF: the first frame flushes the class removal so the browser
	// registers that the animation has stopped; the second frame re-adds the
	// class so the animation restarts cleanly from frame 0. W-21: after the
	// class is set, invoke the scroll-reveal observer so cards on the new page
	// are registered before the user starts scrolling.
	raf := js.Global().Get("requestAnimationFrame")
	raf.Invoke(js.FuncOf(func(_ js.Value, _ []js.Value) any {
		raf.Invoke(js.FuncOf(func(_ js.Value, _ []js.Value) any {
			classList.Call("add", "page-enter")
			triggerScrollReveal()
			return nil
		}))
		return nil
	}))
}

// triggerScrollReveal calls window.cashfluxWonder.observe() (W-21) to register
// new .card elements with the IntersectionObserver after each route change.
// It is a no-op if the JS controller is absent (e.g. script load failed, tests).
func triggerScrollReveal() {
	wonder := js.Global().Get("cashfluxWonder")
	if wonder.IsNull() || wonder.IsUndefined() {
		return
	}
	observe := wonder.Get("observe")
	if observe.IsNull() || observe.IsUndefined() {
		return
	}
	observe.Invoke()
}
