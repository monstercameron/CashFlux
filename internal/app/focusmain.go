// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "syscall/js"

// focusMain moves keyboard and screen-reader focus to the <main> content
// region and scrolls it back to the top. It's called on route changes so SPA
// navigation doesn't strand focus on a control in the screen the user just left;
// landing on <main> (which is labelled) mirrors how a full page load would
// behave. <main> is itself the scroll container, and its scrollTop persists
// across an SPA navigation, so a new screen would otherwise open at the previous
// page's scroll position — reset it to 0 here. The focus move uses preventScroll
// so it doesn't fight the explicit reset.
func focusMain() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	el := doc.Call("getElementById", "main")
	if el.IsNull() || el.IsUndefined() {
		return
	}
	el.Set("scrollTop", 0) // open each screen at the top, not the last page's scroll position
	opts := js.Global().Get("Object").New()
	opts.Set("preventScroll", true)
	el.Call("focus", opts)
}

// setDocumentTitle updates the browser tab/history title to the active screen.
// This is what users see in their tab strip and back-button history, and what
// screen readers announce on a route change, so it should name the current
// screen rather than stay a static "CashFlux".
func setDocumentTitle(title string) {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	doc.Set("title", title)
}
