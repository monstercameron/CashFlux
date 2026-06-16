//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	prefsAtomID  = "app:prefs"
	prefsStoreID = "cashflux:prefs"
)

// UsePrefs returns the shared display-preferences atom, seeded from localStorage
// so week-start and date-format choices survive reloads (the dataset is re-seeded
// each boot, so preferences persist here, not in the store). Screens read it to
// format dates; the settings form writes it back via PersistPrefs.
func UsePrefs() state.Atom[prefs.Prefs] {
	return state.UseAtom(prefsAtomID, loadPrefs())
}

// PersistPrefs saves preferences to localStorage. Call it after writing the atom
// so the choice is remembered across reloads.
func PersistPrefs(p prefs.Prefs) {
	data, err := json.Marshal(p.Normalize())
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", prefsStoreID, string(data))
}

// loadPrefs reads saved preferences from localStorage, falling back to defaults
// when absent or invalid. The result is always normalized.
func loadPrefs() prefs.Prefs {
	v := js.Global().Get("localStorage").Call("getItem", prefsStoreID)
	if v.IsNull() || v.IsUndefined() {
		return prefs.Default()
	}
	var p prefs.Prefs
	if err := json.Unmarshal([]byte(v.String()), &p); err != nil {
		return prefs.Default()
	}
	return p.Normalize()
}
