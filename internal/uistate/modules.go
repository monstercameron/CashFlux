// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/modules"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	hiddenModulesAtomID  = "app:hidden-modules"
	hiddenModulesStoreID = "cashflux:hidden-modules"
)

// UseHiddenModules returns the shared set of hidden screen paths, seeded from
// localStorage so show/hide choices survive reloads. The sidebar reads it to
// filter nav items; the settings toggles write it back via PersistHiddenModules.
func UseHiddenModules() state.Atom[modules.Hidden] {
	return state.UseAtom(hiddenModulesAtomID, loadHiddenModules())
}

// PersistHiddenModules saves the hidden-module set to localStorage. Call it after
// writing the atom so the choice is remembered across reloads.
func PersistHiddenModules(h modules.Hidden) {
	data, err := json.Marshal(h.Normalize())
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", hiddenModulesStoreID, string(data))
}

// loadHiddenModules reads the saved hidden-module set from localStorage, falling
// back to an empty (all-visible) set when absent or invalid. Always normalized.
func loadHiddenModules() modules.Hidden {
	v := js.Global().Get("localStorage").Call("getItem", hiddenModulesStoreID)
	if v.IsNull() || v.IsUndefined() {
		return modules.Hidden{}
	}
	var h modules.Hidden
	if err := json.Unmarshal([]byte(v.String()), &h); err != nil {
		return modules.Hidden{}
	}
	return h.Normalize()
}
