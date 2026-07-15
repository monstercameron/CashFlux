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

// InstitutionsManagerHost is the shell-root flip modal for the AC10 institution
// directory. It reads the institutions-manager atom and, when open, renders the
// list/add/edit body inside a FlipPanel — mounted at the shell root (beside
// AccountGroupsEditHost) so the fixed modal centers on the viewport rather than
// resolving against a transformed bento tile.
func InstitutionsManagerHost() uic.Node {
	open := uistate.UseInstitutionsManager()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("accounts.institutionsManageTitle"),
		Width:    uiw.FlipMediumW,
		Height:   uiw.FlipMediumH,
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.InstitutionsManagerForm, screens.InstitutionsManagerFormProps{OnDone: closeModal}),
	})
}
