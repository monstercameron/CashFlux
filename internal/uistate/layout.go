//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	layoutAtomID  = "dashboard:layout"
	layoutStoreID = "cashflux:layout"
)

// UseLayout returns the shared bento layout atom, seeded from localStorage so a
// rearranged dashboard survives reloads. The Widget shell reads it to place each
// widget; drag-reorder and resize write swapped/resized layouts back (and persist
// via PersistLayout).
func UseLayout() state.Atom[dashlayout.Layout] {
	return state.UseAtom(layoutAtomID, loadLayout())
}

// PersistLayout saves a layout to localStorage. Call it after writing the atom
// so the arrangement is remembered across reloads.
func PersistLayout(l dashlayout.Layout) {
	data, err := json.Marshal(l)
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", layoutStoreID, string(data))
}

// loadLayout reads a saved layout from localStorage, falling back to the default
// arrangement when absent or invalid.
func loadLayout() dashlayout.Layout {
	v := js.Global().Get("localStorage").Call("getItem", layoutStoreID)
	if v.IsNull() || v.IsUndefined() {
		return dashlayout.Default()
	}
	var l dashlayout.Layout
	if err := json.Unmarshal([]byte(v.String()), &l); err != nil || len(l) == 0 {
		return dashlayout.Default()
	}
	return l
}

const dragSrcAtomID = "dashboard:drag-source"

// UseDragSource returns the shared atom holding the id of the widget currently
// being dragged ("" when none), so the drop target knows what to swap with.
func UseDragSource() state.Atom[string] {
	return state.UseAtom(dragSrcAtomID, "")
}
