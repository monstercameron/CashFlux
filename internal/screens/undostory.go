// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// postUndoStory is the compatibility acknowledgement for non-transaction
// screens. Transaction writes use commitUndoable so capture and write cannot
// be separated. Persistence publishes the captured undo point after commit.
func postUndoStory(summary string) {
	auditview.CaptureNow()

	if appstate.Default == nil {
		return
	}
	appstate.Default.RequestDurable(func(result appstate.DurableResult) {
		if result.Err != nil {
			uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
			return
		}
		uistate.PostUndoable(uistate.T("toast.undoStory", summary))
	})
}
