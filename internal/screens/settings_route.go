// SPDX-License-Identifier: MIT

//go:build js && wasm

// This file owns the routed /settings page's registry seam. The tabbed settings
// form itself lives in internal/app beside its flip-modal host (both faces mount
// the same component), and app imports screens — so the registry can't reference
// the form directly. The app package injects it at boot instead.
package screens

import (
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// SettingsView is the injected /settings page body, set by the app package at
// boot (before the router mounts). Nil only during tests that render the
// registry without the app shell.
var SettingsView func() ui.Node

// SettingsScreen is the routed /settings page: the household's full tabbed
// settings surface as a first-class System page in the side nav.
func SettingsScreen() ui.Node {
	if SettingsView == nil {
		return Fragment()
	}
	return SettingsView()
}
