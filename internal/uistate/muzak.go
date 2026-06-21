//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/state"
)

const (
	muzakAtomID  = "app:muzak"
	muzakStoreID = "cashflux:muzak"
)

// UseMuzakEnabled returns the shared on/off atom for the background music, seeded
// from localStorage. Defaults to ON (music plays at low volume until toggled off).
func UseMuzakEnabled() state.Atom[bool] {
	return state.UseAtom(muzakAtomID, loadMuzakEnabled())
}

// PersistMuzakEnabled remembers the music on/off choice across reloads.
func PersistMuzakEnabled(on bool) {
	v := "1"
	if !on {
		v = "0"
	}
	js.Global().Get("localStorage").Call("setItem", muzakStoreID, v)
}

// loadMuzakEnabled reads the saved choice, defaulting to ON when unset.
func loadMuzakEnabled() bool {
	v := js.Global().Get("localStorage").Call("getItem", muzakStoreID)
	if v.IsNull() || v.IsUndefined() {
		return true
	}
	return v.String() != "0"
}
