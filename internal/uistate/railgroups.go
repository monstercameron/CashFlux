// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/GoWebComponents/state"
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

// PersistCollapsedToolGroups saves the collapsed Tools sub-sections.
func PersistCollapsedToolGroups(m map[string]bool) {
	clean := map[string]bool{}
	for k, v := range m {
		if v {
			clean[k] = true
		}
	}
	data, err := json.Marshal(clean)
	if err != nil {
		return
	}
	SettingKVSet(toolGroupsStoreID, string(data))
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
