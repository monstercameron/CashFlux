// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/GoWebComponents/v4/state"
)

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
	a := state.UseAtom(settingsAtomID, SettingsTarget{})
	capturedSettings = a
	settingsCaptured = true
	return a
}

// capturedSettings holds the atom reference captured during a render so that
// OpenGlobalSettings can open the panel from a click handler without calling
// state.UseAtom outside a render (which panics). The shell's SettingsHost calls
// UseSettings every frame, so the capture is always live.
var (
	capturedSettings state.Atom[SettingsTarget]
	settingsCaptured bool
)

// OpenGlobalSettings navigates to the routed /settings page (its default
// tab). Safe from click handlers. This is still the ONE correct way to reach
// Settings from a screen: every "open settings" affordance funnels through
// here so the entry point stays a single decision.
func OpenGlobalSettings() { OpenGlobalSettingsAt("") }

// OpenGlobalSettingsAt navigates straight to the given settings tab's own URL
// ("household", "prefs", "appearance", "alerts", "ai", "cloud", "data",
// "advanced"); "" goes to bare /settings, which itself redirects to the
// default tab (see internal/app's liveSettingsTab). Each tab is a real,
// bookmarkable route ("/settings/cloud") — this used to stash the target tab
// in a one-shot package var for the settings form to consume on mount, before
// tabs had their own URLs at all.
func OpenGlobalSettingsAt(tab string) {
	if tab == "" {
		NavigateTo("/settings")
		return
	}
	NavigateTo("/settings/" + tab)
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

// CurrentDataRevision returns the current data-revision value WITHOUT subscribing (safe
// to call outside a component render). It bumps on every data mutation, so it's a cheap
// cache key for memoizing expensive per-render computations over the dataset: a hit means
// the underlying data hasn't changed. Returns 0 before the first render captures the atom.
func CurrentDataRevision() int {
	if dataRevCaptured {
		return capturedDataRev.Get()
	}
	return 0
}

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
