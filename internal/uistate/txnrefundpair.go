// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

const txnRefundPairAtomID = "transactions:refundPairTarget"

// UseRefundPairTarget returns the shared atom holding the id of the refund
// transaction whose "pair as refund of…" picker is open (XC2). An empty string
// means the picker is closed. A row's action sets it; the shell-root
// RefundPairHost renders the picker modal when it is non-empty.
func UseRefundPairTarget() state.Atom[string] {
	return state.UseAtom(txnRefundPairAtomID, "")
}
