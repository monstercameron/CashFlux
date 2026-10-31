// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// txnMarkProps configures one status mark on a ledger row: the glyph or chip that opens it, what
// the mark means, and the one action that takes the reader to the data behind it.
type txnMarkProps struct {
	TestID  string
	Label   string  // accessible name and tooltip of the trigger
	Content ui.Node // the glyph or chip that is the trigger
	Detail  string  // what the mark says about this charge
	Body    ui.Node // optional extra detail under Detail
	CTA     string  // the button that goes to the data
	OnCTA   func()
}

// txnMark is a status mark that opens a popover with its detail and one call to action. It owns
// its open state, and it stops its clicks so the row's editor does not open with the popover.
func txnMark(props txnMarkProps) ui.Node {
	open := ui.UseState(false)
	anchorID := ui.UseId()
	toggle := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		e.PreventDefault()
		open.Set(!open.Get())
	})
	cta := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		e.PreventDefault()
		open.Set(false)
		if props.OnCTA != nil {
			props.OnCTA()
		}
	})
	close := func() { open.Set(false) }
	body := []any{css.Class("txn-mark-pop"), P(css.Class("txn-mark-detail"), props.Detail)}
	if props.Body != nil {
		body = append(body, props.Body)
	}
	body = append(body, Div(css.Class("txn-mark-actions"),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", props.TestID+"-cta"), OnClick(cta), props.CTA)))
	return Fragment(
		Button(css.Class("txn-mark"), ID(anchorID), Type("button"),
			Attr("data-testid", props.TestID), Attr("aria-label", props.Label), Title(props.Label),
			Attr("aria-haspopup", "dialog"), Attr("aria-expanded", ariaBool(open.Get())), OnClick(toggle),
			props.Content),
		uiw.Overlay(uiw.OverlayProps{
			Open: open.Get(), Kind: uiw.OverlayPopover, Label: props.Label, AnchorID: anchorID,
			OnClose: close, TestID: props.TestID + "-popover", Body: Div(body...),
		}),
	)
}

// txnMarkClearedFilter narrows the ledger to cleared charges, for the cleared mark's call to action.
func txnMarkClearedFilter(setFilter func(func(*uistate.TxFilter))) func() {
	return func() {
		setFilter(func(x *uistate.TxFilter) { x.Cleared = "yes" })
	}
}
