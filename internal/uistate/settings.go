// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/state"
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

// requestedSettingsTab is a one-shot deep-link target for the /settings page's
// tab strip: OpenGlobalSettingsAt sets it, the settings form consumes it on
// mount. Package-level (not an atom) because it must be writable from click
// handlers and is only ever read once.
var requestedSettingsTab string

// OpenGlobalSettings navigates to the routed /settings page. Safe from click
// handlers. This is still the ONE correct way to reach Settings from a screen:
// Settings began as a flip modal with no route, and every "open settings"
// affordance funnels through here so the entry point stays a single decision —
// which is what made the modal→page switch a one-function change.
func OpenGlobalSettings() { OpenGlobalSettingsAt("") }

// OpenGlobalSettingsAt navigates to /settings opened on the given tab
// ("household", "prefs", "alerts", "ai", "cloud", "data", "advanced");
// "" keeps the default. Callers that tell the user to do something on a
// specific tab should land them on that tab.
func OpenGlobalSettingsAt(tab string) {
	requestedSettingsTab = tab
	js.Global().Get("history").Call("pushState", js.Null(), "", RoutePath("/settings"))
	// The history router re-resolves on popstate; pushState alone doesn't fire one.
	js.Global().Call("dispatchEvent", js.Global().Get("PopStateEvent").New("popstate"))
}

// ConsumeRequestedSettingsTab returns the pending deep-link tab once, then
// clears it, so a later plain /settings visit doesn't inherit a stale tab.
func ConsumeRequestedSettingsTab() string {
	t := requestedSettingsTab
	requestedSettingsTab = ""
	return t
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
