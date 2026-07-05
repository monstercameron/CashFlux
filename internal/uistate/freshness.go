// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

const (
	freshnessDismissalsAtomID = "freshness:dismissals"
	freshnessDismissalsStore  = "cashflux:freshness-dismissals"
)

// UseFreshnessDismissals returns the account-level stale-nudge dismissals,
// seeded from localStorage so dismissing the dashboard nudge survives reloads.
func UseFreshnessDismissals() state.Atom[freshness.Dismissals] {
	return state.UseAtom(freshnessDismissalsAtomID, loadFreshnessDismissals())
}

// PersistFreshnessDismissals saves stale-nudge dismissals to localStorage.
func PersistFreshnessDismissals(d freshness.Dismissals) {
	data, err := json.Marshal(d)
	if err != nil {
		return
	}
	kvSet(freshnessDismissalsStore, string(data))
}

func loadFreshnessDismissals() freshness.Dismissals {
	raw := kvGet(freshnessDismissalsStore)
	if raw == "" {
		return freshness.Dismissals{}
	}
	var d freshness.Dismissals
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return freshness.Dismissals{}
	}
	return d
}
