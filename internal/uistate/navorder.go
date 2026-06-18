//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/state"
)

const (
	navOrderAtomID   = "nav:order"
	navOrderStore    = "cashflux:nav-order"
	navDragSrcAtomID = "nav:drag-source"
)

// UseNavDragSource returns the shared atom holding the path of the nav item
// currently being dragged ("" when none), so the drop target knows what to move.
func UseNavDragSource() state.Atom[string] {
	return state.UseAtom(navDragSrcAtomID, "")
}

// UseNavOrder returns the shared atom holding the user's custom primary-nav order
// (a sequence of route paths), seeded from localStorage. Empty means "use the
// default order"; navorder.Apply layers it over the live nav list so newly-added
// or hidden screens are handled gracefully.
func UseNavOrder() state.Atom[[]string] {
	return state.UseAtom(navOrderAtomID, loadNavOrder())
}

// PersistNavOrder saves the custom nav order so a drag-reordered menu survives
// reloads.
func PersistNavOrder(order []string) {
	data, err := json.Marshal(order)
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", navOrderStore, string(data))
}

// loadNavOrder reads the saved nav order, or nil (the default order) when absent
// or unreadable.
func loadNavOrder() []string {
	v := js.Global().Get("localStorage").Call("getItem", navOrderStore)
	if v.IsNull() || v.IsUndefined() {
		return nil
	}
	var order []string
	if err := json.Unmarshal([]byte(v.String()), &order); err != nil {
		return nil
	}
	return order
}
