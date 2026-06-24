// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "syscall/js"

// focusByID moves keyboard focus to the element with the given id, if it is
// present in the document. Row components call it from a UseEffect when an
// inline editor opens, so the cursor lands in the first field without the user
// having to click it (§6.7). An empty id, or one that matches no element, is a
// no-op, so callers can pass a value that is only set while editing.
func focusByID(elemID string) {
	if elemID == "" {
		return
	}
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	el := doc.Call("getElementById", elemID)
	if el.IsNull() || el.IsUndefined() {
		return
	}
	el.Call("focus")
}
