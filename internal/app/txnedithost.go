// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// TxnEditHost mounts the transaction editor at the shell root. Overlay owns
// modal focus, Escape, scroll locking and return to the description or pencil.
func TxnEditHost() uic.Node {
	edit := uistate.UseTxnEdit()
	id := edit.Get()
	savedScroll := uic.UseRef(-1.0)
	// #59 (mobile pass, applies everywhere): opening a row's edit modal must not
	// lose the list's scroll position. Capture the scroll pane's offset when the
	// modal opens and re-assert it after close — the close-time focus restore
	// scrolls the focused row into view, which yanked the pane hundreds of
	// pixels from where the user was. The effect runs on the open/closed
	// transition (hook is unconditional; the early return below only gates
	// rendering).
	openKey := "closed"
	if id != "" {
		openKey = "open"
	}
	uic.UseEffect(func() func() {
		sc := js.Global().Get("document").Call("querySelector", "main.cf-scroll")
		if !sc.Truthy() {
			return nil
		}
		if id != "" {
			savedScroll.Set(sc.Get("scrollTop").Float())
			return nil
		}
		if savedScroll.Get() >= 0 {
			top := savedScroll.Get()
			savedScroll.Set(-1)
			sc.Set("scrollTop", top)
			// Focus restoration lands a beat later and scrolls again; one
			// delayed re-assert wins without fighting user scrolling.
			var cb js.Func
			cb = js.FuncOf(func(js.Value, []js.Value) any {
				if s2 := js.Global().Get("document").Call("querySelector", "main.cf-scroll"); s2.Truthy() {
					s2.Set("scrollTop", top)
				}
				return nil
			})
			timer := js.Global().Call("setTimeout", cb, 120)
			return func() {
				js.Global().Call("clearTimeout", timer)
				cb.Release()
			}
		}
		return nil
	}, openKey)
	close := func() { uistate.SetTxnEdit("") }
	return uic.CreateElement(txnEditOverlay, txnEditOverlayProps{
		Open:    id != "",
		Body:    uic.CreateElement(screens.TransactionEditForm, screens.TransactionEditFormProps{TxnID: id, OnDone: close}),
		Delete:  uic.CreateElement(screens.TxnEditDeleteFooter, screens.TransactionEditFormProps{TxnID: id, OnDone: close}),
		OnClose: close,
	})
}

type txnEditOverlayProps struct {
	Open         bool
	Body, Delete uic.Node
	OnClose      func()
}

func txnEditOverlay(props txnEditOverlayProps) uic.Node {
	cancel := uic.UseEvent(func() {
		if props.OnClose != nil {
			props.OnClose()
		}
	})
	return uiw.Overlay(uiw.OverlayProps{
		Open: props.Open, Kind: uiw.OverlayModal, Title: uistate.T("transactions.editTitle"),
		TestID: "txn-edit-dialog", OnClose: props.OnClose, Body: props.Body,
		Footer: Div(css.Class("txn-form-footer"),
			props.Delete,
			Div(css.Class("txn-form-footer-actions"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-edit-cancel"), OnClick(cancel), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("form", "txn-edit-form"),
					Attr("data-testid", "txn-edit-save"), uistate.T("action.save")),
			),
		),
	})
}
