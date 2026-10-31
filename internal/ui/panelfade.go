// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"syscall/js"

	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// panelFadeMs is how long a fading panel takes to open or close. It matches the
// .fade-panel timing in the styles.
const panelFadeMs = 250

// UsePanelFade reports whether a panel should stay in the DOM. It is true while the
// panel is open and for the fade-out after it closes, so closing can animate rather than
// vanish. Reduced motion skips the fade, and the panel leaves at once. Call it
// unconditionally at a stable render position.
func UsePanelFade(open bool) bool {
	mounted := uic.UseState(false)
	uic.UseEffect(func() func() {
		if open {
			if !mounted.Get() {
				mounted.Set(true)
			}
			return nil
		}
		if !mounted.Get() {
			return nil
		}
		if prefersReducedMotion() {
			mounted.Set(false)
			return nil
		}
		return afterDelay(panelFadeMs, func() { mounted.Set(false) })
	}, open, mounted.Get())
	return mounted.Get()
}

// prefersReducedMotion reports whether the reader has asked for less motion.
func prefersReducedMotion() bool {
	match := js.Global().Get("matchMedia")
	return match.Truthy() && match.Invoke("(prefers-reduced-motion: reduce)").Get("matches").Bool()
}

// afterDelay runs fn once after ms milliseconds. The returned cancel stops it if it has
// not run yet, and releases the callback either way.
func afterDelay(ms int, fn func()) func() {
	global := js.Global()
	done := false
	var timer js.Value
	var callback js.Func
	callback = js.FuncOf(func(js.Value, []js.Value) any {
		done = true
		callback.Release()
		fn()
		return nil
	})
	timer = global.Call("setTimeout", callback, ms)
	return func() {
		if !done {
			global.Call("clearTimeout", timer)
			callback.Release()
		}
	}
}

// FadeClass is a class for content that fades and grows in when it opens, and fades out
// when it closes. Pair it with UsePanelFade, which keeps the content mounted for the fade-out.
func FadeClass(base string, open bool) string {
	if open {
		return base + " fade-panel"
	}
	return base + " fade-panel is-leaving"
}
