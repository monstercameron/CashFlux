// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// UseOnline returns the shared atom holding the browser's online/offline state
// (true = online). The top-bar OfflineIndicator renders it; app boot wiring sets
// it from navigator.onLine and the window online/offline events. Defaults true so
// the app never flashes an "offline" warning before the real state is read.
func UseOnline() state.Atom[bool] {
	return state.UseAtom("app:online", true)
}

// onlineAtom is captured from the indicator's render so SetOnline notifies the
// exact instance the component subscribed with (mirrors the dialog/notice seam).
var (
	onlineAtom  state.Atom[bool]
	onlineReady bool
)

// CaptureOnline registers the atom the OfflineIndicator renders with. Call from
// that component each render.
func CaptureOnline(a state.Atom[bool]) {
	onlineAtom, onlineReady = a, true
}

// SetOnline updates the shared online state (no-op until the indicator has
// mounted and captured its atom). Called from the window online/offline handlers.
func SetOnline(v bool) {
	if onlineReady {
		onlineAtom.Set(v)
	}
}
