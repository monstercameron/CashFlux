// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

const quickAddAtomID = "quickadd:open"

// UseQuickAdd returns the shared atom tracking whether the quick-add transaction
// panel is open. The top bar's "+ Add" button sets it true; the quick-add host
// (at the shell root) reads it to render the flip panel.
//
// Reading it also captures the atom so SetQuickAdd can open the panel from outside
// a component render (keyboard shortcuts, command-palette actions) — calling the
// UseQuickAdd hook itself from those non-render callbacks panics (hook-outside-
// component).
func UseQuickAdd() state.Atom[bool] {
	a := state.UseAtom(quickAddAtomID, false)
	capturedQuickAdd = a
	quickAddCaptured = true
	return a
}

var (
	capturedQuickAdd state.Atom[bool]
	quickAddCaptured bool
)

// SetQuickAdd opens or closes the quick-add panel from outside a component render
// (global callbacks). No-op until the quick-add host has rendered once.
func SetQuickAdd(open bool) {
	if quickAddCaptured {
		capturedQuickAdd.Set(open)
	}
}
