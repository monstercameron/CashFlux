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

// StatementImportHost mounts the "Import statement" (AI PDF import) flip modal at the
// shell root. It's a LARGE modal because the reviewed transactions render as a table.
// The Back body owns its own action bar (NoFooter). Mounting at the shell root keeps
// the fixed modal clear of the bento tile transform that would clip it.
func StatementImportHost() uic.Node {
	open := uistate.UseStatementImportOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("statementimport.title"),
		Width:    "900px",
		Height:   "660px",
		NoFooter: true,
		OnClose:  func() { open.Set(false) },
		Back:     uic.CreateElement(screens.StatementImportBody, struct{}{}),
	})
}
