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

// AccountGroupsEditHost is the shell-root flip modal for creating or editing an
// account group (AC1): a name field + a checkable list of accounts, plus Delete when
// editing. Driven by the account-group-edit atom ("" = closed, "new" = create, or a
// group id = edit). Mounted at the shell root so the fixed modal centers on the
// viewport rather than resolving against a transformed bento tile. The form owns
// Save/Delete and clears the atom via OnDone.
func AccountGroupsEditHost() uic.Node {
	edit := uistate.UseAccountGroupEdit()
	gid := edit.Get()
	if gid == "" {
		return Fragment()
	}
	closeModal := func() { edit.Set("") }
	title, saveLabel := uistate.T("accounts.newGroup"), uistate.T("accounts.createGroup")
	if gid != "new" {
		title, saveLabel = uistate.T("accounts.editGroup"), uistate.T("accounts.saveGroup")
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:        title,
		Width:        uiw.FlipMediumW,
		Height:       uiw.FlipMediumH,
		FormID:       "account-group-form",
		SaveLabel:    saveLabel,
		SaveTestID:   "group-save",
		CancelTestID: "group-cancel",
		OnClose:      closeModal,
		Back:         uic.CreateElement(screens.AccountGroupsForm, screens.AccountGroupsFormProps{ID: gid, OnDone: closeModal}),
	})
}
