// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

const txnSplitAtomID = "ui:txnSplit"

// UseTxnSplit returns the shared atom holding the id of the transaction whose
// split-into-categories modal is open. An empty string means no modal is open.
// The transactions table sets it (the row's ⋯ menu); TxnSplitHost reads it to
// render the split editor inside a FlipPanel.
//
// Reading the atom here also captures it in the package-level var so SetTxnSplit
// can be called from outside a component render (e.g. a row-menu callback that
// fires post-render) without hitting hook-outside-component.
func UseTxnSplit() state.Atom[string] {
	a := state.UseAtom(txnSplitAtomID, "")
	capturedTxnSplit = a
	txnSplitCaptured = true
	return a
}

var (
	capturedTxnSplit state.Atom[string]
	txnSplitCaptured bool
)

// SetTxnSplit opens or closes the split-into-categories modal from outside a
// component render. Pass a transaction id to open, "" to close. No-op until
// TxnSplitHost has rendered once (capturing the atom).
func SetTxnSplit(id string) {
	if txnSplitCaptured {
		capturedTxnSplit.Set(id)
	}
}
