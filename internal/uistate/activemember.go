// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/state"
)

const (
	activeMemberAtomID = "household:activeMember"
	activeMemberStore  = "cashflux:active-member"
)

// UseActiveMember returns the shared atom that tracks which household member's
// perspective the app is currently scoped to. An empty string means "Everyone"
// (the default, full-household view). A non-empty value is a member ID and
// causes member-aware screens to scope their figures and lists to that member.
// The value is persisted to localStorage so the chosen view survives reloads.
func UseActiveMember() state.Atom[string] {
	a := state.UseAtom(activeMemberAtomID, loadActiveMember())
	capturedActiveMember = a
	activeMemberCaptured = true
	return a
}

var (
	capturedActiveMember state.Atom[string]
	activeMemberCaptured bool
)

// SetActiveMember changes the active-member view from outside a component
// render (e.g. the member-switcher callback) and persists the choice. Calling
// the UseActiveMember hook from a global callback would panic (hook-outside-
// component); route through here instead. No-op until the switcher has
// rendered once.
func SetActiveMember(memberID string) {
	if !activeMemberCaptured {
		return
	}
	capturedActiveMember.Set(memberID)
	PersistActiveMember(memberID)
}

// PersistActiveMember saves the active member ID to localStorage. An empty
// string is stored as "" to distinguish "Everyone" from "never set".
func PersistActiveMember(memberID string) {
	js.Global().Get("localStorage").Call("setItem", activeMemberStore, memberID)
}

// loadActiveMember reads the saved active member from localStorage, defaulting
// to "" (Everyone) when absent.
func loadActiveMember() string {
	v := js.Global().Get("localStorage").Call("getItem", activeMemberStore)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}
