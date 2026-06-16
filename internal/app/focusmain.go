//go:build js && wasm

package app

import "syscall/js"

// focusMain moves keyboard and screen-reader focus to the <main> content
// region. It's called on route changes so SPA navigation doesn't strand focus
// on a control in the screen the user just left; landing on <main> (which is
// labelled and at the top of the new screen) mirrors how a full page load would
// behave. preventScroll keeps the focus move from yanking the scroll position,
// since main is already the scroll container's top.
func focusMain() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	el := doc.Call("getElementById", "main")
	if el.IsNull() || el.IsUndefined() {
		return
	}
	opts := js.Global().Get("Object").New()
	opts.Set("preventScroll", true)
	el.Call("focus", opts)
}
