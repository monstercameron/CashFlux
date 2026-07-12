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

// DuplicatesHost mounts the transactions "Review duplicates" panel as a double-wide flip
// modal at the shell root — the same shape as the Import modal. It used to take over the
// transactions page as a full-width in-place sub-view (TxnViewDuplicates); a shell-root
// mount keeps the fixed modal clear of the bento tile transform that would clip it.
//
// CloseOnly footer: the panel resolves each duplicate group in place (merge / delete are
// immediate, undoable actions), so there's nothing to stage — a single Done button, not a
// Cancel/Save pair.
func DuplicatesHost() uic.Node {
	open := uistate.UseDuplicatesModalOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("transactions.dupReviewBtn"),
		Width:     "900px",
		Height:    "660px",
		CloseOnly: true,
		OnClose:   func() { open.Set(false) },
		Back:      uic.CreateElement(screens.DuplicatesPanelBody, struct{}{}),
	})
}
