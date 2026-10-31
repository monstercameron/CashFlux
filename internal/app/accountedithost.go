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

// AccountEditHost is mounted at the shell root, beside TxnEditHost. It reads the account editor
// atom and renders the matching editor (edit, update value, reconcile or transfer) in a
// FlipPanel. When the atom is empty it renders nothing.
//
// The panel sits at the shell root rather than inside the account row. A transformed ancestor
// makes a position:fixed backdrop resolve against that ancestor instead of the viewport, and an
// account row sits under such ancestors, which pushed an in-row modal off-centre. The shell root
// has no transformed ancestor, so the modal centres on the viewport.
//
// The panel has no footer of its own. The editor form supplies Save and Cancel and calls OnDone
// when it finishes, which clears the atom.
func AccountEditHost() uic.Node {
	edit := uistate.UseAccountEdit()
	e := edit.Get()
	if e.ID == "" || e.Mode == "" {
		return Fragment()
	}
	closeModal := uistate.CloseAccountEdit

	title := uistate.T("accounts.editTitle")
	switch e.Mode {
	case uistate.AcctEditModeSetBal:
		// Use the same wording as the row button ("Update value" for estimated-asset types,
		// "Update balance" for cash accounts), so the title and the button agree.
		title = screens.AccountUpdateActionLabel(e.ID)
	case uistate.AcctEditModeReconcile:
		title = uistate.T("accounts.reconcileTitle")
	case uistate.AcctEditModeTransfer:
		title = uistate.T("accounts.transferTitle")
	}

	// NoFooter and FlushBody: the form supplies its own Save and Cancel bar, pinned below a
	// scrolling field region. The header close button, Escape and a backdrop click call OnClose.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     title,
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   closeModal,
		Back:      uic.CreateElement(screens.AccountEditForm, screens.AccountEditFormProps{AccountID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}
