// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
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
		// The footer advances only the visible preview or reviewed draft.
		FormID:    screens.ImportModalFormID,
		SaveLabel: uistate.T("action.done"),
		OnClose:   func() { screens.CancelImportPanel(); open.Set(false) },
		Back:      uic.CreateElement(screens.ImportPanelBody, struct{}{}),
	})
}
