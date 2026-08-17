// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

// UseTxnSearchPending returns the atom that is true while a typed ledger search
// is still inside its debounce window.
//
// C619 tracked this inside the toolbar tile, which is where the "Searching…"
// note lives — but the rows it is a warning ABOUT are in a different tile, and a
// warning that cannot reach the thing it is warning about is only half a
// warning. C628 asks for the stale rows to be marked and their actions
// unavailable until the query lands, so the flag has to be shared state, not
// component state.
//
// It is deliberately NOT persisted: a debounce window is an in-flight moment, and
// a reload that restored "searching" would be claiming an update that no longer
// exists.
func UseTxnSearchPending() state.Atom[bool] {
	return state.UseAtom("transactions:searchPending", false)
}
