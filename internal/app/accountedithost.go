// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
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

	title, width, height := "", "440px", "500px"
	switch e.Mode {
	case uistate.AcctEditModeSetBal:
		title, width, height = uistate.T("accounts.updateBalance"), "420px", "440px"
	case uistate.AcctEditModeReconcile:
		title, width, height = uistate.T("accounts.reconcileTitle"), "460px", "560px"
	case uistate.AcctEditModeTransfer:
		title, width, height = uistate.T("accounts.transferTitle"), "440px", "520px"
	default:
		title, width, height = uistate.T("accounts.editTitle"), "480px", "600px"
	}

	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     title,
		Width:     width,
		Height:    height,
		CloseOnly: true,
		OnClose:   closeModal,
		Back:      uic.CreateElement(screens.AccountEditForm, screens.AccountEditFormProps{AccountID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}
