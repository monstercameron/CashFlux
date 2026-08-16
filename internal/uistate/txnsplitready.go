// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

const txnSplitReadyAtomID = "ui:txnSplitReady"

// UseTxnSplitReady returns the shared atom holding whether the open split editor's
// draft is currently saveable.
//
// C566: the split editor owns the draft, but its Save button lives in the hosting
// FlipPanel's pinned footer — so the footer had no way to know the draft was
// unfinished and rendered an enabled Save over a split that could only fail. The
// editor publishes its verdict here (from an effect, never the render body) and the
// host reads it into FlipPanelProps.SaveDisabled, so the button and the draft agree.
//
// It defaults to true so a panel that has not yet reported (the first frame, or any
// future host that does not publish) is never stuck with a dead Save.
func UseTxnSplitReady() state.Atom[bool] {
	a := state.UseAtom(txnSplitReadyAtomID, true)
	capturedTxnSplitReady = a
	txnSplitReadyCaptured = true
	return a
}

var (
	capturedTxnSplitReady state.Atom[bool]
	txnSplitReadyCaptured bool
)

// SetTxnSplitReady publishes the open split draft's saveability from outside a
// component render (the editor's post-render effect). No-op until the atom has been
// read once.
func SetTxnSplitReady(ok bool) {
	if txnSplitReadyCaptured && capturedTxnSplitReady.Get() != ok {
		capturedTxnSplitReady.Set(ok)
	}
}
