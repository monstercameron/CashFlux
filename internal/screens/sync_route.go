// SPDX-License-Identifier: MIT

//go:build js && wasm

// This file owns the routed /sync page's registry seam. Like /settings, the page
// body lives in internal/app (beside the sync engine + prefs it drives), and app
// imports screens — so the registry can't reference the body directly. The app
// package injects it at boot.
package screens

import (
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// SyncView is the injected /sync page body, set by the app package at boot
// (before the router mounts). Nil only during tests that render the registry
// without the app shell.
var SyncView func() ui.Node

// SyncScreen is the routed /sync page: a focused, top-level surface to connect a
// backend, toggle sync on/off, and see live sync status — promoted out of the
// Settings → Cloud tab so it's one click from the side nav.
func SyncScreen() ui.Node {
	if SyncView == nil {
		return Fragment()
	}
	return SyncView()
}
