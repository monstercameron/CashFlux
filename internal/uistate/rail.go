// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/GoWebComponents/v4/state"
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
	a := state.UseAtom(railCollapsedAtomID, loadRailCollapsed())
	capturedRail = a
	railCaptured = true
	return a
}

var (
	capturedRail state.Atom[bool]
	railCaptured bool
)

// ToggleRailCollapsed flips the sidebar collapsed state from outside a component
// render (keyboard shortcut / command palette) and persists it. Calling the
// UseRailCollapsed hook from such a callback panics (hook-outside-component), so
// global handlers must route through here. Returns the new state; no-op (false)
// until the sidebar has rendered once.
func ToggleRailCollapsed() bool {
	if !railCaptured {
		return false
	}
	v := !capturedRail.Get()
	capturedRail.Set(v)
	PersistRailCollapsed(v)
	return v
}

// PersistRailCollapsed saves whether the sidebar is collapsed.
func PersistRailCollapsed(collapsed bool) {
	v := "0"
	if collapsed {
		v = "1"
	}
	SettingKVSet(railCollapsedStore, v)
}

// loadRailCollapsed reads the saved collapsed state, defaulting to expanded.
func loadRailCollapsed() bool {
	return SettingKVGet(railCollapsedStore) == "1"
}
