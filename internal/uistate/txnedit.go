// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

const txnEditAtomID = "ui:txnEdit"

// UseTxnEdit returns the shared atom holding the id of the transaction whose
// edit modal is open. An empty string means no modal is open. The transactions
// table sets it (clicking a row); TxnEditHost reads it to render the matching
// edit form inside a FlipPanel.
//
// Reading the atom here also captures it in the package-level var so SetTxnEdit
// can be called from outside a component render (e.g. a row-click callback that
// fires post-render) without hitting hook-outside-component.
func UseTxnEdit() state.Atom[string] {
	a := state.UseAtom(txnEditAtomID, "")
	capturedTxnEdit = a
	txnEditCaptured = true
	return a
}

var (
	capturedTxnEdit state.Atom[string]
	txnEditCaptured bool
)

// SetTxnEdit opens or closes the transaction edit modal from outside a component
// render. Pass a transaction id to open, "" to close. No-op until TxnEditHost has
// rendered once (capturing the atom).
func SetTxnEdit(id string) {
	if txnEditCaptured {
		capturedTxnEdit.Set(id)
	}
}
