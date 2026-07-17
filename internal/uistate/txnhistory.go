// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// capturedTxnHistory lets a transaction row's "History" kebab item open the
// per-transaction history flip modal from a click handler without calling
// UseAtom outside a render (#63). Mirrors the payee-clean / split-modal seams.
var (
	capturedTxnHistory state.Atom[string]
	txnHistoryCaptured bool
)

// UseTxnHistory returns the atom holding the transaction id whose history modal
// is open ("" = closed). TxnHistoryHost (shell root) reads it and renders the
// flip modal; a row's kebab item sets it.
func UseTxnHistory() state.Atom[string] {
	a := state.UseAtom("txn:history", "")
	capturedTxnHistory = a
	txnHistoryCaptured = true
	return a
}

// SetTxnHistory opens the history modal for a transaction (or closes it with "").
func SetTxnHistory(txnID string) {
	if txnHistoryCaptured {
		capturedTxnHistory.Set(txnID)
	}
}
