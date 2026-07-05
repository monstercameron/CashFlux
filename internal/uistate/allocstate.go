// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// The allocate strategy (profile / mode / buffer / cap / criterion weights) lives in shared
// atoms rather than the screen's local state, because it is edited in a shell-root FLIP MODAL
// (AllocProfileHost) that is a separate component tree from the main /allocate surface — both
// read and write these atoms, and the ranked plan re-ranks live as they change. profile/mode
// seed from the persisted AllocConfig; buffer/cap/weights are seeded once by the screen (which
// has the base-currency precision) — see the allocSeeded flag.

// UseAllocProfileOpen drives the strategy flip modal ("Adjust strategy"): true = open.
func UseAllocProfileOpen() state.Atom[bool] { return state.UseAtom("alloc:profileOpen", false) }

// UseAllocProfileSel is the active ranking profile key (seeded from the saved plan).
func UseAllocProfileSel() state.Atom[string] {
	return state.UseAtom("alloc:profile", AllocConfigGet().Profile)
}

// UseAllocModeSel is the split mode: "weighted" or "fill" (seeded from the saved plan).
func UseAllocModeSel() state.Atom[string] { return state.UseAtom("alloc:mode", AllocConfigGet().Mode) }

// UseAllocReserveStr / UseAllocMaxPerStr are the buffer + per-destination cap input strings.
func UseAllocReserveStr() state.Atom[string] { return state.UseAtom("alloc:reserve", "") }
func UseAllocMaxPerStr() state.Atom[string]  { return state.UseAtom("alloc:maxper", "") }

// UseAllocW* are the five editable criterion weights (seeded from the active profile).
func UseAllocWReturns() state.Atom[string]   { return state.UseAtom("alloc:wReturns", "1") }
func UseAllocWStability() state.Atom[string] { return state.UseAtom("alloc:wStability", "1") }
func UseAllocWLiquidity() state.Atom[string] { return state.UseAtom("alloc:wLiquidity", "1") }
func UseAllocWDebt() state.Atom[string]      { return state.UseAtom("alloc:wDebt", "1") }
func UseAllocWGoal() state.Atom[string]      { return state.UseAtom("alloc:wGoal", "1") }

// UseAllocSeeded marks that the screen has seeded the buffer/cap/weights atoms once (from the
// persisted plan + active profile), so it doesn't clobber live edits on every re-render.
func UseAllocSeeded() state.Atom[bool] { return state.UseAtom("alloc:seeded", false) }
