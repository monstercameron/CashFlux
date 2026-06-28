// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — active-scope atom.
//
// UseActiveScope / SetActiveScope replace the narrower UseActiveMember atom as
// the single source of truth for which accounts, institutions, types, and
// member-owners a report view covers. The scope is persisted as JSON to the KV
// store under the key cashflux:active-scope.
//
// Migration: on first read, if cashflux:active-member is set (the legacy key)
// and cashflux:active-scope is absent, the member ID is promoted into
// Scope.Owners and the old key is cleared. This means the app transitions
// transparently: existing installs that had a member filter keep it,
// represented as a single-owner scope.
//
// The legacy UseActiveMember hook (activemember.go) is left untouched to avoid
// breaking concurrent WIP on other files. Use ActiveMemberFromScope() to
// derive the equivalent member-ID string from the active scope when a
// single-member view is needed.
package uistate

import (
	"encoding/json"

	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/CashFlux/internal/scope"
)

const (
	activeScopeAtomID = "household:activeScope"
	activeScopeStore  = "cashflux:active-scope"
)

// UseActiveScope returns the shared atom that tracks the current report scope.
// An IsAll() scope (all fields empty) means "no restriction — show everything",
// which is the default household view. The atom is persisted so the chosen
// scope survives page reloads.
func UseActiveScope() state.Atom[scope.ReportScope] {
	a := state.UseAtom(activeScopeAtomID, loadActiveScope())
	capturedActiveScope = a
	activeScopeCaptured = true
	return a
}

var (
	capturedActiveScope  state.Atom[scope.ReportScope]
	activeScopeCaptured  bool
)

// SetActiveScope changes the active scope from outside a component render
// (e.g. a scope-selector callback or the migration path) and persists the
// choice. No-op until UseActiveScope has been called at least once by a
// mounted component.
func SetActiveScope(s scope.ReportScope) {
	if !activeScopeCaptured {
		return
	}
	capturedActiveScope.Set(s)
	persistActiveScope(s)
}

// persistActiveScope writes the scope JSON to the KV store.
func persistActiveScope(s scope.ReportScope) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	kvSet(activeScopeStore, string(b))
}

// loadActiveScope reads the persisted scope from the KV store.
//
// Migration: if cashflux:active-scope is absent but cashflux:active-member is
// set, the member ID is promoted into Owners and the old key is cleared so the
// new scope becomes the single source of truth.
func loadActiveScope() scope.ReportScope {
	raw := kvGet(activeScopeStore)
	if raw != "" {
		var s scope.ReportScope
		if err := json.Unmarshal([]byte(raw), &s); err == nil {
			return s
		}
		// Corrupt data — fall through to empty scope (no migration needed).
		return scope.ReportScope{}
	}

	// cashflux:active-scope is absent. Check for the legacy active-member key.
	memberID := kvGet(activeMemberStore) // defined in activemember.go
	if memberID != "" {
		// Promote legacy member filter into a single-owner scope.
		migrated := scope.ReportScope{Owners: []string{memberID}}
		persistActiveScope(migrated)
		kvDelete(activeMemberStore)
		return migrated
	}

	return scope.ReportScope{}
}

// ActiveMemberFromScope derives the single-member-ID string from the current
// persisted active scope, for callers that need the legacy "active member ID"
// string. Returns "" when the scope is all-inclusive or contains multiple
// owners (i.e. can't be reduced to one member ID).
func ActiveMemberFromScope() string {
	raw := kvGet(activeScopeStore)
	if raw == "" {
		return ""
	}
	var s scope.ReportScope
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return ""
	}
	if len(s.Owners) == 1 {
		return s.Owners[0]
	}
	return ""
}
