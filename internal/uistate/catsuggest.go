// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

// capturedCatSuggest lets a transaction row's "Categorize this" kebab item open
// the categorize flip modal (SM-2) from a click handler without calling UseAtom
// outside a render. Mirrors the payee-clean / task-edit shell-root modal seams.
var (
	capturedCatSuggest state.Atom[string]
	catSuggestCaptured bool
)

// UseCatSuggest returns the atom holding the transaction id whose categorize
// modal is open ("" = closed). CatSuggestHost (shell root) reads it and renders
// the flip modal; a row's kebab item sets it. Calling it in a render also
// captures the atom for SetCatSuggest / CloseCatSuggest.
func UseCatSuggest() state.Atom[string] {
	a := state.UseAtom("txn:catSuggest", "")
	capturedCatSuggest = a
	catSuggestCaptured = true
	return a
}

// SetCatSuggest opens the categorize modal for a transaction (or closes it with "").
func SetCatSuggest(txnID string) {
	if catSuggestCaptured {
		capturedCatSuggest.Set(txnID)
	}
}

// CloseCatSuggest closes the categorize modal.
func CloseCatSuggest() {
	if catSuggestCaptured {
		capturedCatSuggest.Set("")
	}
}
