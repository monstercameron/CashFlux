// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnFocusRowHint tells the focus helper how to make a row EXIST before it goes
// looking for one.
//
// On a paged view every row of the page is in the DOM and a query finds it. On the
// virtualized "All" view only the rows near the current scroll offset are
// rendered, so a query for a row further down finds nothing however long it
// waits — the row is not late, it is absent, and it will stay absent until the
// scroller moves. IndexOf gives the helper the row's place in the frame so it can
// move the scroller there first.
type txnFocusRowHint struct {
	// Virtual is true when the table is rendering a window rather than every row.
	Virtual bool
	// RowHeight is the fixed row height the virtual window measures against.
	RowHeight int
	// Scroller is the CSS selector for the element that actually scrolls.
	Scroller string
	// IndexOf resolves a transaction id to its position in the current frame.
	IndexOf func(id string) (int, bool)
}

// useTxnFocusRow lands the ledger on the row a returning breadcrumb named (C605).
//
// It runs as an effect keyed on the request, not during render: the row has to
// exist in the DOM before it can be scrolled to, and the request arrives with the
// navigation that mounts the table.
//
// The row is scrolled to CENTER rather than into-view-minimum, because "into view"
// against a sticky table header puts the row underneath it — technically visible,
// actually hidden. It is also focused, so a keyboard user resumes where they were
// and a screen reader announces the row rather than silently repositioning.
//
// The request is consumed once. Left set, it would re-grab the viewport on every
// later re-render of the table — sort, select, page — and fight the user for
// control of their own scroll position.
func useTxnFocusRow(hint txnFocusRowHint) {
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

		// Virtualized: put the scroller where the row will render, THEN look for it.
		// Without this the retry below spins for a second against a row the window
		// was never going to draw, and the feature silently does nothing on exactly
		// the long list it exists to serve.
		if hint.Virtual && hint.IndexOf != nil && hint.RowHeight > 0 {
			if idx, ok := hint.IndexOf(id); ok {
				if sc := doc.Call("querySelector", hint.Scroller); sc.Truthy() {
					target := idx*hint.RowHeight - sc.Get("clientHeight").Int()/2
					if target < 0 {
						target = 0
					}
					sc.Set("scrollTop", target)
				}
			}
		}

		// Retried across a few frames rather than tried once. The table paints in
		// stages — the frame is hydrated, the rows render, deferred chips mount — and
		// a single rAF after the effect regularly lands before the row exists. One
		// missed frame would make the whole feature look unimplemented.
		//
		// It gives up after ~1s. Giving up is silent on purpose: the row can be
		// genuinely gone (the filter no longer matches it, or it was deleted while
		// the user was away), and the user asked to come back to a LIST, which they
		// have. An error about a row they may not be looking for would be noise.
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
