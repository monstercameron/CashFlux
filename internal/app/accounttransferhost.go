// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// AccountTransferHost is mounted at the shell root (beside AccountEditHost). It reads
// the page-level transfer-open atom and renders the "Transfer money" form inside a
// modal sheet. When the atom is false no overlay is shown.
//
// Mounting at the shell root — rather than as an inline tile on the accounts surface —
// keeps the sheet's fixed backdrop on the viewport instead of resolving against a
// transformed bento tile, matching the account row editors.
//
// The sheet has no footer: the form supplies its own Cancel/Transfer action row. The
// header close button, Escape, and a click on the backdrop all dismiss via OnClose.
func AccountTransferHost() uic.Node {
	open := uistate.UseAcctTransferOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	// The form needs the loaded app; until then the panel says the page is not ready.
	var form uic.Node
	if app := appstate.Default; app != nil {
		form = uic.CreateElement(screens.AccountPageTransferForm, screens.AccountPageTransferProps{
			App: app, OnDone: closeModal,
		})
	} else {
		form = P(css.Class("empty"), uistate.T("common.notReady"))
	}
	return uiw.Sheet(uiw.OverlayProps{
		Open:    true,
		Title:   uistate.T("accounts.transferTitle"),
		OnClose: closeModal,
		Body:    form,
	})
}
