//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/GoWebComponents/state"
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
	js.Global().Get("localStorage").Call("setItem", freshnessDismissalsStore, string(data))
}

func loadFreshnessDismissals() freshness.Dismissals {
	v := js.Global().Get("localStorage").Call("getItem", freshnessDismissalsStore)
	if v.IsNull() || v.IsUndefined() {
		return freshness.Dismissals{}
	}
	var d freshness.Dismissals
	if err := json.Unmarshal([]byte(v.String()), &d); err != nil {
		return freshness.Dismissals{}
	}
	return d
}
