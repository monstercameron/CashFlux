// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// useTxnFocusRow lands the ledger on the row a returning breadcrumb named (C605).
//
// It runs as an effect keyed on the request, not during render: the row has to
// exist in the DOM before it can be scrolled to, and the request arrives with the
// navigation that mounts the table. One animation frame after the effect gives the
// table its paint.
//
// The row is scrolled to CENTER rather than into-view-minimum, because "into view"
// against a sticky table header puts the row underneath it — technically visible,
// actually hidden. It is also focused, so a keyboard user resumes where they were
// and a screen reader announces the row rather than silently repositioning.
//
// The request is consumed once. Left set, it would re-grab the viewport on every
// later re-render of the table — sort, select, page — and fight the user for
// control of their own scroll position.
func useTxnFocusRow() {
	req := uistate.UseTxnFocusRow().Get()
	ui.UseEffect(func() func() {
		if req == "" {
			return nil
		}
		id := uistate.TakeTxnFocusRow()
		if id == "" {
			return nil
		}
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return nil
		}
		var cb js.Func
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			cb.Release()
			row := doc.Call("querySelector", "[data-testid=\"txn-row-"+id+"\"]")
			if !row.Truthy() {
				// The row is not in the current page or the filter no longer matches
				// it. Nothing to do — and nothing to say: the user asked to go back to
				// a list, and they have it.
				return nil
			}
			opts := js.Global().Get("Object").New()
			opts.Set("block", "center")
			opts.Set("behavior", "auto")
			row.Call("scrollIntoView", opts)
			row.Call("setAttribute", "tabindex", "-1")
			row.Call("focus")
			return nil
		})
		js.Global().Call("requestAnimationFrame", cb)
		return nil
	}, "txn-focus-row:"+req)
}
