//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/state"
)

const (
	railCollapsedAtomID = "rail:collapsed"
	railCollapsedStore  = "cashflux:rail-collapsed"
)

// UseRailCollapsed returns the shared atom coordinating the collapsible sidebar:
// the top bar's menu button toggles it and the sidebar reads it to switch to
// icon-only mode. It is seeded from localStorage so the choice survives reloads
// (C20). Keyed globally so both components stay in sync.
func UseRailCollapsed() state.Atom[bool] {
	return state.UseAtom(railCollapsedAtomID, loadRailCollapsed())
}

// PersistRailCollapsed saves whether the sidebar is collapsed.
func PersistRailCollapsed(collapsed bool) {
	v := "0"
	if collapsed {
		v = "1"
	}
	js.Global().Get("localStorage").Call("setItem", railCollapsedStore, v)
}

// loadRailCollapsed reads the saved collapsed state, defaulting to expanded.
func loadRailCollapsed() bool {
	v := js.Global().Get("localStorage").Call("getItem", railCollapsedStore)
	if v.IsNull() || v.IsUndefined() {
		return false
	}
	return v.String() == "1"
}
