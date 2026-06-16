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
	return SettingsTarget{Kind: "global", Title: "Settings"}
}

const dataRevAtomID = "data:revision"

// UseDataRevision returns the shared atom bumped whenever the whole dataset is
// replaced (import, load-sample, wipe). Screens that read store data directly
// read this too, so they re-render after a bulk data change.
func UseDataRevision() state.Atom[int] {
	return state.UseAtom(dataRevAtomID, 0)
}
