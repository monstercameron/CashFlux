// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// AccountEditHost is mounted at the shell root (beside TxnEditHost). It reads the
// account-editor atom and renders the matching editor (edit / update-balance /
// reconcile / transfer) inside a FlipPanel modal. When the atom is empty no overlay
// is shown.
//
// Mounting at the shell root — rather than inside the account row — is the whole
// point: a row lives under transformed bento/tile ancestors (`.w`, `.card`, and the
// app-enter wrapper all carry a transform), which would make the modal's
// position:fixed backdrop resolve against the tile instead of the viewport, so an
// in-row modal rendered off-centre. Here there is no transformed ancestor, so the
// modal centres on the viewport.
//
// The FlipPanel is CloseOnly (its footer is a single "Close"); the editor form owns
// its own Save/Cancel and calls OnDone on completion, which clears the atom.
func AccountEditHost() uic.Node {
	edit := uistate.UseAccountEdit()
	e := edit.Get()
	if e.ID == "" || e.Mode == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseAccountEdit() }

	// Standard sizes: the short "update balance" form is Small, the rest Medium.
	title, width, height := uistate.T("accounts.editTitle"), uiw.FlipMediumW, uiw.FlipMediumH
	switch e.Mode {
	case uistate.AcctEditModeSetBal:
		title, width, height = uistate.T("accounts.updateBalance"), uiw.FlipSmallW, uiw.FlipSmallH
	case uistate.AcctEditModeReconcile:
		title = uistate.T("accounts.reconcileTitle")
	case uistate.AcctEditModeTransfer:
		title = uistate.T("accounts.transferTitle")
	}

	// NoFooter: the editor form supplies its own (sticky-pinned) Save/Cancel action row,
	// so the modal isn't double-chromed with a redundant Close footer. The header ✕,
	// Escape, and backdrop-click still dismiss via OnClose.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    title,
		Width:    width,
		Height:   height,
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.AccountEditForm, screens.AccountEditFormProps{AccountID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}
