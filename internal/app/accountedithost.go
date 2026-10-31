// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// AccountEditHost is mounted at the shell root, beside TxnEditHost. It reads the account
// editor atom and renders the matching editor (edit, update value, reconcile or transfer)
// in a FlipPanel modal. When the atom is empty it renders nothing.
//
// The modal lives at the shell root rather than inside the account row. A row sits under
// transformed ancestors (.w, .card and the app-enter wrapper all carry a transform), and a
// transform makes a position:fixed backdrop resolve against that ancestor instead of the
// viewport, which pushed an in-row modal off-centre. Here there is no transformed ancestor,
// so the modal centres on the viewport.
//
// The panel has no footer. The editor form supplies its own Save and Cancel row and calls
// OnDone when it finishes, which clears the atom.
func AccountEditHost() uic.Node {
	edit := uistate.UseAccountEdit()
	e := edit.Get()
	if e.ID == "" || e.Mode == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseAccountEdit() }

	// Every account editor uses the medium panel size.
	title, width, height := uistate.T("accounts.editTitle"), uiw.FlipMediumW, uiw.FlipMediumH
	switch e.Mode {
	case uistate.AcctEditModeSetBal:
		// Match the row button's wording ("Update value" for estimated-asset types,
		// "Update balance" for cash accounts) so the title isn't at odds with it.
		title = screens.AccountUpdateActionLabel(e.ID)
	case uistate.AcctEditModeReconcile:
		title = uistate.T("accounts.reconcileTitle")
	case uistate.AcctEditModeTransfer:
		title = uistate.T("accounts.transferTitle")
	}

	// NoFooter: the editor form supplies its own Save and Cancel row, so the panel has no
	// footer of its own. The header close button, Escape and a backdrop click call OnClose.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     title,
		Width:     width,
		Height:    height,
		NoFooter:  true,
		FlushBody: true, // the form splits into a scrolling field region + a pinned .modal-foot
		OnClose:   closeModal,
		Back:      uic.CreateElement(screens.AccountEditForm, screens.AccountEditFormProps{AccountID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}
