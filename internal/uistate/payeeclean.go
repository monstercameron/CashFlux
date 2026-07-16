// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// capturedPayeeClean lets a transaction row's "Clean up name" kebab item open the
// payee-cleanup flip modal from a click handler without calling UseAtom outside a
// render. Mirrors the task-edit / other shell-root modal seams.
var (
	capturedPayeeClean state.Atom[string]
	payeeCleanCaptured bool
)

// UsePayeeClean returns the atom holding the transaction id whose payee-cleanup modal
// is open ("" = closed). PayeeCleanHost (shell root) reads it and renders the flip
// modal; a row's kebab item sets it. Calling it in a render also captures the atom for
// SetPayeeClean / ClosePayeeClean.
func UsePayeeClean() state.Atom[string] {
	a := state.UseAtom("txn:payeeClean", "")
	capturedPayeeClean = a
	payeeCleanCaptured = true
	return a
}

// SetPayeeClean opens the payee-cleanup modal for a transaction (or closes it with "").
func SetPayeeClean(txnID string) {
	if payeeCleanCaptured {
		capturedPayeeClean.Set(txnID)
	}
}

// ClosePayeeClean closes the payee-cleanup modal.
func ClosePayeeClean() {
	if payeeCleanCaptured {
		capturedPayeeClean.Set("")
	}
}
