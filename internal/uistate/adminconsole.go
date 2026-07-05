// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// UseAdminConsoleAvailable returns the shared atom that tracks whether the
// current bearer token has admin access on the configured backend. False until
// the boot probe receives HTTP 200 from GET /v1/admin/overview; stays false on
// 401, 403, network error, or when no backend is configured. Nav gating in
// shell.go reads this atom to hide the /admin route from non-admin users.
func UseAdminConsoleAvailable() state.Atom[bool] {
	return state.UseAtom("app:adminConsoleAvailable", false)
}

// adminConsoleAtom is captured from the first render that calls
// UseAdminConsoleAvailable so SetAdminConsoleAvailable can post from outside a
// component (boot probe goroutine). Mirrors the online/sampledata seam.
var (
	adminConsoleAtom  state.Atom[bool]
	adminConsoleReady bool
)

// CaptureAdminConsole registers the atom so SetAdminConsoleAvailable can write
// it from outside a component render. Call from Sidebar (or any component that
// calls UseAdminConsoleAvailable) each render.
func CaptureAdminConsole(a state.Atom[bool]) {
	adminConsoleAtom, adminConsoleReady = a, true
}

// SetAdminConsoleAvailable updates the admin-available atom from outside a
// component render (e.g. the boot-probe goroutine). No-op until the atom has
// been captured by at least one component render.
func SetAdminConsoleAvailable(v bool) {
	if adminConsoleReady {
		adminConsoleAtom.Set(v)
	}
}
