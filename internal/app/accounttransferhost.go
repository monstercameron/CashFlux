// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// AccountTransferHost is mounted at the shell root (beside AccountEditHost). It reads
// the page-level transfer-open atom and renders the "Transfer money" form inside a
// FlipPanel modal. When the atom is false no overlay is shown.
//
// Mounting at the shell root — rather than as an inline tile on the accounts surface —
// keeps the modal's position:fixed backdrop centered on the viewport instead of
// resolving against a transformed bento tile, matching the account row editors.
//
// NoFooter: the form supplies its own Cancel/Transfer action row, so the modal isn't
// double-chromed. The header ✕, Escape, and backdrop-click all dismiss via OnClose.
func AccountTransferHost() uic.Node {
	open := uistate.UseAcctTransferOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("accounts.transferTitle"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true, // the form splits into a scrolling field region + a pinned .modal-foot
		OnClose:   closeModal,
		Back: uic.CreateElement(screens.AccountPageTransferForm, screens.AccountPageTransferProps{
			App: appstate.Default, OnDone: closeModal,
		}),
	})
}
