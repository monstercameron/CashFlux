// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

var syncConflictHandle interface {
	Get() bool
	Set(bool)
}

// openSyncConflict opens review without choosing or replacing either copy.
func openSyncConflict() {
	if syncConflictHandle != nil {
		syncConflictHandle.Set(true)
	}
}

// SyncConflictHost adapts the existing conflict commands to the shared overlay.
// Keeping both is the default presentation action; it leaves the saved copy intact.
func SyncConflictHost() uic.Node {
	open := state.UseAtom("sync:conflict:open", false)
	syncConflictHandle = open
	return uic.CreateElement(cloudConflictReview, cloudConflictReviewProps{
		Open: open.Get(), OnClose: func() { open.Set(false) },
		OnKeepLocal: resolveConflictKeepLocal,
		OnUseServer: func() {
			uistate.ConfirmModalLabeled(uistate.T("cloud1.useServerConfirm"), uistate.T("cloud1.useServer"), true, func(ok bool) {
				if ok {
					resolveConflictUseServer()
					open.Set(false)
				}
			})
		},
	})
}
