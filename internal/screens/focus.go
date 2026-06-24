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

// pendingRowDeleteFocus carries a captured row index from a delete-button click
// (where focus is still on the row) across an async confirm modal to the actual
// delete handler, so focus can be restored to the next row afterwards (§6.7).
// -1 means "nothing captured". Single-threaded wasm, so a plain var is safe.
var pendingRowDeleteFocus = -1

// captureRowDeleteFocus records the focused row's index now (call it in a delete
// button's click handler, before any confirm modal steals focus).
func captureRowDeleteFocus(listSel, rowSel string) {
	pendingRowDeleteFocus = rowIndexOfActive(listSel, rowSel)
}

// consumeRowDeleteFocus returns the captured index and resets it to -1.
func consumeRowDeleteFocus() int {
	i := pendingRowDeleteFocus
	pendingRowDeleteFocus = -1
	return i
}

// rowIndexOfActive returns the 0-based index, among the elements matching rowSel
// inside the first listSelector, of the row that currently contains keyboard
// focus — or -1 if focus isn't inside the list. Call it synchronously at the
// start of a delete handler (while the about-to-be-removed row still holds
// focus) to capture where focus should land afterwards (§6.7).
func rowIndexOfActive(listSelector, rowSel string) int {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return -1
	}
	active := doc.Get("activeElement")
	if !active.Truthy() {
		return -1
	}
	list := doc.Call("querySelector", listSelector)
	if !list.Truthy() {
		return -1
	}
	rows := list.Call("querySelectorAll", rowSel)
	n := rows.Get("length").Int()
	for i := 0; i < n; i++ {
		if rows.Call("item", i).Call("contains", active).Bool() {
			return i
		}
	}
	return -1
}

// focusRowAfterDelete restores focus after a row delete so it is never dropped to
// <body> (§6.7). It waits one animation frame (so the re-render has repainted the
// list), then focuses a control in the row now occupying deletedIndex — clamped to
// the last row when the deleted row was last — preferring the row's delete button
// so a keyboard user can continue down the list. When the list is now empty it
// focuses the list container if focusable, else the document body's add affordance
// is left to the caller. A deletedIndex < 0 (focus wasn't in the list) is a no-op.
func focusRowAfterDelete(listSelector, rowSel string, deletedIndex int) {
	if deletedIndex < 0 {
		return
	}
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	var cb js.Func
	cb = js.FuncOf(func(this js.Value, args []js.Value) any {
		cb.Release()
		list := doc.Call("querySelector", listSelector)
		if !list.Truthy() {
			return nil
		}
		rows := list.Call("querySelectorAll", rowSel)
		n := rows.Get("length").Int()
		if n == 0 {
			return nil
		}
		idx := deletedIndex
		if idx >= n {
			idx = n - 1
		}
		row := rows.Call("item", idx)
		// Prefer the delete button (or any focusable control) within the row.
		target := row.Call("querySelector", "[aria-label*='elete'], .btn-del, button, [tabindex]")
		if !target.Truthy() {
			target = row
		}
		if target.Get("focus").Type() == js.TypeFunction {
			target.Call("focus")
		}
		return nil
	})
	// setTimeout (not rAF) so this runs AFTER a closing confirm modal's own
	// focus-restore on unmount — otherwise the modal would steal focus back to
	// <body> right after we set it.
	js.Global().Call("setTimeout", cb, 60)
}

// captureRowFocus handles screens with MULTIPLE row lists (e.g. accounts: assets,
// liabilities, archived). It locates the focused row's list (the nearest ancestor
// matching listSel) and its position, and returns a restore func to call after the
// delete + re-render: on the next animation frame it focuses the equivalent row in
// the same list (by list index + row index, clamped), so focus stays in context
// instead of dropping to <body> (§6.7). If focus isn't in such a row, the returned
// func is a no-op.
func captureRowFocus(listSel, rowSel string) func() {
	noop := func() {}
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return noop
	}
	active := doc.Get("activeElement")
	if !active.Truthy() {
		return noop
	}
	list := active.Call("closest", listSel)
	if !list.Truthy() {
		return noop
	}
	lists := doc.Call("querySelectorAll", listSel)
	listIdx := -1
	for i := 0; i < lists.Get("length").Int(); i++ {
		if lists.Call("item", i).Call("isSameNode", list).Bool() {
			listIdx = i
			break
		}
	}
	rows := list.Call("querySelectorAll", rowSel)
	rowIdx := -1
	for i := 0; i < rows.Get("length").Int(); i++ {
		if rows.Call("item", i).Call("contains", active).Bool() {
			rowIdx = i
			break
		}
	}
	if listIdx < 0 || rowIdx < 0 {
		return noop
	}
	return func() {
		var cb js.Func
		cb = js.FuncOf(func(this js.Value, args []js.Value) any {
			cb.Release()
			ls := doc.Call("querySelectorAll", listSel)
			if listIdx >= ls.Get("length").Int() {
				return nil
			}
			rs := ls.Call("item", listIdx).Call("querySelectorAll", rowSel)
			n := rs.Get("length").Int()
			if n == 0 {
				return nil
			}
			idx := rowIdx
			if idx >= n {
				idx = n - 1
			}
			row := rs.Call("item", idx)
			target := row.Call("querySelector", "[aria-label*='elete'], .btn-del, button, [tabindex]")
			if !target.Truthy() {
				target = row
			}
			if target.Get("focus").Type() == js.TypeFunction {
				target.Call("focus")
			}
			return nil
		})
		js.Global().Call("setTimeout", cb, 60)
	}
}
