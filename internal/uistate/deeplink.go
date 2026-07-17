// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// DeepLinkFocus carries a CSS selector for an element the app should scroll to and
// briefly flash after a cross-page jump — so a notification lands on the exact
// account or budget it's about, not just the owning page. Empty = nothing to focus.
// It uses the captured-atom seam (like TaskAddSeed) so a click handler on one screen
// can set it without holding a hook.
var (
	capturedDeepLinkFocus state.Atom[string]
	deepLinkFocusCaptured bool
)

// UseDeepLinkFocus returns the deep-link focus atom. The always-mounted
// DeepLinkFocusHost calls this so it can react; calling it also captures the atom
// for SetDeepLinkFocus.
func UseDeepLinkFocus() state.Atom[string] {
	a := state.UseAtom("deeplink:focus", "")
	capturedDeepLinkFocus = a
	deepLinkFocusCaptured = true
	return a
}

// SetDeepLinkFocus asks the app to scroll to and flash the element matching
// selector after the next navigation. Safe to call from a click handler.
func SetDeepLinkFocus(selector string) {
	if deepLinkFocusCaptured {
		capturedDeepLinkFocus.Set(selector)
	}
}
