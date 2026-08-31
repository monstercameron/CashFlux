// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/v5/state"
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
	// One persist, not two: SettingKVSet already writes on the leading edge of a
	// burst, so the RequestPersist that followed exported the whole dataset a
	// second time for the same toggle.
	SettingKVSetDeferred(railCollapsedStore, v)
	mirrorRailClass(collapsed)
}

// loadRailCollapsed reads the saved collapsed state, defaulting to expanded.
func loadRailCollapsed() bool {
	collapsed := SettingKVGet(railCollapsedStore) == "1"
	// Seeding doubles as the boot-time sync for the <html> mirror class (every
	// later change flows through PersistRailCollapsed, which mirrors again).
	mirrorRailClass(collapsed)
	return collapsed
}

// mirrorRailClass reflects the rail state onto <html> as `cf-rail-c` — the hook
// the styles package's content-width breakpoint helpers key on
// (styles/breakpoints.go): a collapsed rail leaves the content pane 182px
// wider, and layout rules need to know which width they actually have. Class
// absent = expanded, the conservative default (layouts compact sooner, nothing
// clips). Idempotent; safe to call on every read.
func mirrorRailClass(collapsed bool) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	root := doc.Get("documentElement")
	if !root.Truthy() {
		return
	}
	method := "remove"
	if collapsed {
		method = "add"
	}
	root.Get("classList").Call(method, "cf-rail-c")
}
