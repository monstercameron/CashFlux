//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/widgetvis"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	hiddenWidgetsAtomID  = "dashboard:hidden-widgets"
	hiddenWidgetsStoreID = "cashflux:hidden-widgets"
)

// UseHiddenWidgets returns the shared set of hidden dashboard widget instance ids,
// seeded from localStorage so show/hide choices survive reloads. The dashboard
// reads it to skip hidden tiles (and reflow around them); the Widget Manager
// writes it back via PersistHiddenWidgets.
func UseHiddenWidgets() state.Atom[widgetvis.Set] {
	return state.UseAtom(hiddenWidgetsAtomID, loadHiddenWidgets())
}

// PersistHiddenWidgets saves the hidden-widget set to localStorage. Call it after
// writing the atom so the choice is remembered across reloads.
func PersistHiddenWidgets(s widgetvis.Set) {
	data, err := json.Marshal(s.Normalize())
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", hiddenWidgetsStoreID, string(data))
}

// loadHiddenWidgets reads the saved hidden-widget set, falling back to an empty
// (all-visible) set when absent or invalid. Always normalized.
func loadHiddenWidgets() widgetvis.Set {
	v := js.Global().Get("localStorage").Call("getItem", hiddenWidgetsStoreID)
	if v.IsNull() || v.IsUndefined() {
		return widgetvis.Set{}
	}
	var s widgetvis.Set
	if err := json.Unmarshal([]byte(v.String()), &s); err != nil {
		return widgetvis.Set{}
	}
	return s.Normalize()
}
