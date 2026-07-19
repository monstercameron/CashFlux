// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// postUndoStory tells the reversal story at the moment of risk (C364). After a
// bulk mutation lands it (1) captures an undo point immediately — so the diff
// stack is armed the instant the toast appears, not up to one autosave tick
// later — and (2) posts an UNDOABLE toast whose text spells out the reversal
// path: keyboard undo (Ctrl+Z) or the full history in Activity.
//
// summary is the per-operation, already-translated description (e.g. "12
// transactions recategorized"); the trailing "· Undo (Ctrl+Z) · View in
// Activity" clause is appended uniformly via the toast.undoStory catalog key so
// every bulk site reads the same. The toast surface renders a working "Undo"
// button off the undoable flag; the transactions undo bar and the /activity
// route provide the working links.
//
// It is safe to call from any completion handler in the screens package —
// bulk recategorize/edit, import commit, duplicate merge/resolution, and the
// rules bulk apply — without each site importing auditview or duplicating the
// capture-then-toast dance.
//
// TODO(W1/C364): the fully-uniform *clickable* "View in Activity" link belongs
// in the shared toast component (internal/app/toast.go), which is outside this
// lane. Once that lands, the toast can navigate to /activity directly for every
// site instead of only via the transactions undo bar. Tracked in the lane report.
func postUndoStory(summary string) {
	auditview.CaptureNow()
	uistate.PostUndoable(uistate.T("toast.undoStory", summary))
}
