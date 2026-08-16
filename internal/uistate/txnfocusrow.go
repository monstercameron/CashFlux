// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

const txnFocusRowAtomID = "transactions:focusRow"

// UseTxnFocusRow returns the shared atom naming a ledger row that should be
// scrolled to and focused once the table has painted — "" for none.
//
// C605: returning from a side trip restored the FILTER, which brings the right
// list back but drops you at its top. On the 122-row list this feature exists to
// serve, that is most of what losing your place means: the user goes to Rules to
// handle one charge, comes back, and has to find that charge again by eye.
//
// It is consumed once and cleared, like the quick-add kind seed. A row focus that
// persisted would re-grab the scroll position on every later re-render of the
// table — on sort, on selection, on paging — and fight the user for control of
// the viewport.
func UseTxnFocusRow() state.Atom[string] {
	a := state.UseAtom(txnFocusRowAtomID, "")
	capturedTxnFocusRow = a
	txnFocusRowCaptured = true
	return a
}

var (
	capturedTxnFocusRow state.Atom[string]
	txnFocusRowCaptured bool
)

// SetTxnFocusRow asks the ledger to land on a row, from outside a render.
func SetTxnFocusRow(id string) {
	if txnFocusRowCaptured {
		capturedTxnFocusRow.Set(id)
	}
}

// TakeTxnFocusRow reads and clears the request.
func TakeTxnFocusRow() string {
	if !txnFocusRowCaptured {
		return ""
	}
	v := capturedTxnFocusRow.Get()
	if v != "" {
		capturedTxnFocusRow.Set("")
	}
	return v
}
