// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

// SettingsTarget identifies which settings panel (if any) is open. Kind is "" for
// closed, "widget" for a per-widget panel (ID/Title identify the widget), or
// "global" for the household/global settings panel.
type SettingsTarget struct {
	Kind  string
	ID    string
	Title string
}

// Open reports whether a settings panel should be shown.
func (t SettingsTarget) Open() bool { return t.Kind != "" }

const settingsAtomID = "settings:target"

// UseSettings returns the shared atom tracking the open settings panel. The
// settings host (at the shell root) reads it to render the FlipPanel; widget
// gears and the household card write it.
func UseSettings() state.Atom[SettingsTarget] {
	return state.UseAtom(settingsAtomID, SettingsTarget{})
}

// Widget builds a target that opens a per-widget settings panel.
func Widget(id, title string) SettingsTarget {
	return SettingsTarget{Kind: "widget", ID: id, Title: title}
}

// Global builds a target that opens the global settings panel.
func Global() SettingsTarget {
	return SettingsTarget{Kind: "global", Title: T("settings.panelTitle")}
}

const dataRevAtomID = "data:revision"

// UseDataRevision returns the shared atom bumped whenever the whole dataset is
// replaced (import, load-sample, wipe). Screens that read store data directly
// read this too, so they re-render after a bulk data change.
//
// Reading it also captures the atom's get/set closures into a package var so that
// BumpDataRevision can bump it from outside a component render (the hook itself
// must run during render, but the captured closures are safe to call anywhere).
func UseDataRevision() state.Atom[int] {
	a := state.UseAtom(dataRevAtomID, 0)
	capturedDataRev = a
	dataRevCaptured = true
	return a
}

var (
	capturedDataRev state.Atom[int]
	dataRevCaptured bool
)

// BumpDataRevision increments the shared data-revision atom from outside a render
// — for global callbacks such as undo/redo or post-decrypt hydration that replace
// the dataset without being inside a component. It is a no-op until at least one
// component that reads UseDataRevision has rendered (always true after first paint,
// since the dashboard reads it).
func BumpDataRevision() {
	if dataRevCaptured {
		capturedDataRev.Set(capturedDataRev.Get() + 1)
	}
}
