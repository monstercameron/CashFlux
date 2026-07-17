// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

const smartStripOpenAtomID = "smartstrip:open"

// UseSmartStripOpen returns the shared atom tracking whether the current page's
// Smart-insights strip is expanded. The top bar's Smart trigger (icon + count)
// sets it; the in-page strip reads it to render the full insight card. Session
// state only — it resets to collapsed on navigation (the Shell closes it on
// route change) so the decision-first default holds page to page.
//
// Reading it also captures the atom so SetSmartStripOpen can close the strip
// from outside a component render (the Shell's route-change effect).
func UseSmartStripOpen() state.Atom[bool] {
	a := state.UseAtom(smartStripOpenAtomID, false)
	capturedSmartStripOpen = a
	smartStripOpenCaptured = true
	return a
}

var (
	capturedSmartStripOpen state.Atom[bool]
	smartStripOpenCaptured bool
)

// SetSmartStripOpen opens or collapses the Smart strip from outside a component
// render (e.g. the Shell's navigation effect). No-op until a reader has rendered.
func SetSmartStripOpen(open bool) {
	if smartStripOpenCaptured {
		capturedSmartStripOpen.Set(open)
	}
}
