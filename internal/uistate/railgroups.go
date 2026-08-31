// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/GoWebComponents/v5/state"
)

const (
	toolGroupsAtomID  = "rail:tool-groups"
	toolGroupsStoreID = "cashflux:rail-tool-groups"
)

// UseCollapsedToolGroups returns the shared set of collapsed Tools sub-sections
// (C67), keyed by sub-group id; a true value means collapsed. Seeded from
// localStorage so the choice survives reloads.
func UseCollapsedToolGroups() state.Atom[map[string]bool] {
	return state.UseAtom(toolGroupsAtomID, loadCollapsedToolGroups())
}

// PersistCollapsedToolGroups saves the collapsed Tools sub-sections. Explicit
// false values are kept too: sections default to COLLAPSED (#60 lean sidebar),
// so "expanded" is a deliberate choice that must survive reloads.
func PersistCollapsedToolGroups(m map[string]bool) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	// SettingKVSet ALREADY persists on the leading edge of a burst (RH-PERSIST1),
	// so the RequestPersist that used to follow it here was a second full dataset
	// export for the same write. On a household with ~3,300 transactions each
	// export is ~500ms of JSON serialisation on the main thread, which made
	// opening a menu folder cost about a second — measured 1.07s per click, of
	// which this line was half. The comment that justified it ("only reaches disk
	// on the autosave tick") predates the leading-edge persist and was no longer
	// true.
	SettingKVSetDeferred(toolGroupsStoreID, string(data))
}

func loadCollapsedToolGroups() map[string]bool {
	raw := SettingKVGet(toolGroupsStoreID)
	if raw == "" {
		return map[string]bool{}
	}
	var m map[string]bool
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]bool{}
	}
	return m
}
