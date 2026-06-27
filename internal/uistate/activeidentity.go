// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/GoWebComponents/state"
)

const (
	activeIdentityAtomID = "household:activeIdentity"
	activeIdentityStore  = "cashflux:active-identity"
)

// UseActiveIdentity returns the shared atom that tracks which household member
// is currently OPERATING the app — i.e. whose role governs what actions are
// permitted. This is distinct from UseActiveMember, which is a view-filter
// (whose data is shown). An empty string means no identity is set yet; callers
// should treat that the same as the Owner (permissive fallback) until the atom
// is wired to real member data. The value is persisted so it survives reloads.
//
// Identity vs filter:
//   - ActiveIdentity = "who is acting" (role enforcement, write gating)
//   - ActiveMember   = "whose data is shown" (view scoping, display filter)
//
// The two may or may not be equal; they are managed independently.
func UseActiveIdentity() state.Atom[string] {
	a := state.UseAtom(activeIdentityAtomID, loadActiveIdentity())
	capturedActiveIdentity = a
	activeIdentityCaptured = true
	return a
}

var (
	capturedActiveIdentity state.Atom[string]
	activeIdentityCaptured bool
)

// SetActiveIdentity changes the active-identity member from outside a component
// render (e.g. the identity-switcher callback) and persists the choice. No-op
// until UseActiveIdentity has been called at least once.
func SetActiveIdentity(memberID string) {
	if !activeIdentityCaptured {
		return
	}
	capturedActiveIdentity.Set(memberID)
	PersistActiveIdentity(memberID)
}

// PersistActiveIdentity saves the active-identity member ID to the KV store.
func PersistActiveIdentity(memberID string) {
	kvSet(activeIdentityStore, memberID)
}

// ActiveIdentityID returns the currently persisted active-identity member ID
// (or "" when none has been set). This can be read outside a component render.
func ActiveIdentityID() string {
	return loadActiveIdentity()
}

// loadActiveIdentity reads the saved identity from the KV store, defaulting to
// "" (no override — Owner permissive fallback applies in appstate).
func loadActiveIdentity() string {
	return kvGet(activeIdentityStore)
}
