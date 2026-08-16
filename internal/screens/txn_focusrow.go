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
		// Retried across a few frames rather than tried once. The table paints in
		// stages — the frame is hydrated, the rows render, deferred chips mount — and
		// a single rAF after the effect regularly lands before the row exists. One
		// missed frame would make the whole feature look unimplemented.
		//
		// It gives up after ~1s. The case it cannot serve is the virtualized "All"
		// view (pageSize <= 0 and more than txnVirtualizeThreshold rows), where only
		// the rows near the current scroll offset exist in the DOM at all and a row
		// far down the list will never appear on its own. Restoring that needs the
		// row's INDEX to drive the virtual scroller, which the table has and this does
		// not — worth doing, not worth faking. On every paged view (the default 25,
		// and any explicit size) the persisted filter restores the page too, so the
		// row is on it.
		const tries = 60
		attempt := 0
		var cb js.Func
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			row := doc.Call("querySelector", "[data-testid=\"txn-row-"+id+"\"]")
			if !row.Truthy() {
				attempt++
				if attempt < tries {
					js.Global().Call("requestAnimationFrame", cb)
					return nil
				}
				cb.Release()
				return nil
			}
			cb.Release()
			opts := js.Global().Get("Object").New()
			// CENTER, not "nearest": the ledger's header is sticky, so scrolling a row
			// just into view parks it underneath the header — present, and invisible.
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
