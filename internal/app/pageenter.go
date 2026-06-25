// SPDX-License-Identifier: MIT

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
// W-10 progressive enhancement: when document.startViewTransition is available
// and motion is on, the class-toggle is wrapped in startViewTransition so the
// browser uses its view-transition machinery for the new-page appearance.
// Because GWC's virtual-DOM render swap fires before this UseEffect hook runs,
// the outgoing page snapshot is already replaced by the time we call
// startViewTransition — so this does not produce a true old→new cross-fade.
// What it does produce is the W-9 fade-rise driven through the View Transitions
// API, which is still a correct progressive enhancement: the CSS
// ::view-transition-* keyframes (wonder-xfade-in) apply, and browsers that
// don't support the API fall back silently to the plain double-rAF path.
//
// After the animation class is toggled it also calls
// window.cashfluxWonder.observe() (W-21) so the IntersectionObserver picks up
// newly rendered .card elements on the incoming page.
//
// The function is a no-op when the element is not found (e.g. during tests).
// Motion gating (data-wonder="off", prefers-reduced-motion) is handled inside
// window.cashfluxWonder.crossFade (wonder.js) so the Go side stays thin.
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

	// applyEnter re-adds the .page-enter class and fires the W-21 observer.
	// It is called either directly (fallback rAF path) or via startViewTransition.
	applyEnter := func() {
		classList.Call("add", "page-enter")
		triggerScrollReveal()
	}

	// W-10: delegate to cashfluxWonder.crossFade when available so the browser
	// can optionally use view-transition machinery. crossFade handles all motion
	// gating and falls back to a direct applyFn() call when the API is absent or
	// motion is off, so the rAF below is only reached when wonder.js is absent.
	wonder := js.Global().Get("cashfluxWonder")
	if !wonder.IsNull() && !wonder.IsUndefined() {
		crossFade := wonder.Get("crossFade")
		if !crossFade.IsNull() && !crossFade.IsUndefined() {
			// crossFade may invoke cb ASYNCHRONOUSLY: document.startViewTransition runs its
			// update callback on a later microtask, not inline. A defer-Release here would free
			// cb before the browser calls it, producing "call to released function" on every
			// route change. So cb releases ITSELF after it runs — correct for both the async
			// view-transition path and the synchronous direct-applyFn fallback (the spec
			// guarantees the update callback is always invoked exactly once).
			var cb js.Func
			cb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
				applyEnter()
				cb.Release()
				return nil
			})
			crossFade.Invoke(cb)
			return
		}
	}

	// Fallback: double-rAF without the View Transitions API. First frame flushes the class
	// removal; second restarts the keyframe. Each callback releases itself after firing so no
	// js.Func leaks accumulate across route changes.
	raf := js.Global().Get("requestAnimationFrame")
	var first, second js.Func
	second = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		applyEnter()
		second.Release()
		return nil
	})
	first = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		raf.Invoke(second)
		first.Release()
		return nil
	})
	raf.Invoke(first)
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
