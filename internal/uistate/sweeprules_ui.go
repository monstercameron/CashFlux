// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// UseSweepRulesOpen is the atom selecting whether the /accounts sweep-rules
// manager modal is open (AC7).
func UseSweepRulesOpen() state.Atom[bool] {
	return state.UseAtom("accounts:sweepRulesOpen", false)
}
