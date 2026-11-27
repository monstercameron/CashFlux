// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/syncengine"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v8/state"
	uic "github.com/monstercameron/GoWebComponents/v8/ui"
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

// SyncConflictHost adapts the conflict checkpoint comparison and live-applied
// resolutions (Keep Both/Merge, Keep Mine, Use Server) to the shared overlay.
func SyncConflictHost() uic.Node {
	open := state.UseAtom("sync:conflict:open", false)
	_ = state.UseAtom(syncRevAtomID, 0).Get()
	syncConflictHandle = open
	cmp, hasDiff := activeV2ConflictComparison()
	return uic.CreateElement(cloudConflictReview, cloudConflictReviewProps{
		Open:       open.Get(),
		HasDiff:    hasDiff,
		Comparison: cmp,
		OnClose:    func() { open.Set(false) },
		OnMerge: func() {
			if resolveV2ConflictMerge(syncengine.PreferLocal) {
				open.Set(false)
			}
		},
		OnKeepLocal: func() {
			resolveConflictKeepLocal()
			open.Set(false)
		},
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
