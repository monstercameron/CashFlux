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

// ImportPanelHost mounts the transactions "Import" panel (CSV paste, receipt vision,
// draft review, import history) as a double-wide flip modal at the shell root — the
// same shape as the Import-statement modal. It used to take over the transactions page
// as a full-width in-place sub-view (TxnViewImport); a shell-root mount keeps the fixed
// modal clear of the bento tile transform that would clip it. It's a large/double-wide
// modal because the import panel is content-heavy (draft review renders a table).
func ImportPanelHost() uic.Node {
	open := uistate.UseImportPanelOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:  uistate.T("transactions.importBtn"),
		Width:  "900px",
		Height: "660px",
		// Standard pinned footer: Cancel dismisses; the primary natively submits the
		// panel's hidden form (ImportModalFormID), whose handler commits the ready import
		// (reviewed draft or pasted CSV) and closes. FormID mode doesn't auto-close, so
		// the commit runs first and a failed commit keeps the modal open. Labeled "Done"
		// rather than "Save": across the wizard the primary commits-what's-ready-and-
		// closes (nothing to "save" while the user is still choosing an import method),
		// and each source form carries its own explicit Import action.
		FormID:    screens.ImportModalFormID,
		SaveLabel: uistate.T("action.done"),
		OnClose: func() { open.Set(false) },
		Back:    uic.CreateElement(screens.ImportPanelBody, struct{}{}),
	})
}
