// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/GoWebComponents/v4/state"
)

// savedViewDismissKey is the localStorage key for the set of dismissed saved-view
// threshold notices. Each entry is a savedtxnview.DismissalKey (viewID@threshold),
// so re-raising a threshold produces a fresh, undismissed notice.
const savedViewDismissKey = "cashflux:saved-view-threshold-dismissals"

// UseSavedViewsOpen returns the shared atom selecting whether the transactions
// toolbar's "Views" popover is open. Its data (the live count/total per view) is
// computed lazily when this becomes true, not on every ledger render.
func UseSavedViewsOpen() state.Atom[bool] {
	return state.UseAtom("transactions:savedViewsOpen", false)
}

// UseSaveViewFormOpen returns the shared atom selecting whether the "Save current
// view…" naming form (inside the Views popover) is open.
func UseSaveViewFormOpen() state.Atom[bool] {
	return state.UseAtom("transactions:saveViewFormOpen", false)
}

// UseSavedViewThresholdDismissals returns the atom holding the set of dismissed
// saved-view threshold-notice keys, seeded from localStorage so a dismissal
// survives reloads.
func UseSavedViewThresholdDismissals() state.Atom[map[string]bool] {
	return state.UseAtom("transactions:savedViewDismissals", loadSavedViewDismissals())
}

// DismissSavedViewThreshold marks a saved-view threshold notice dismissed (by its
// dismissal key) and persists the updated set.
func DismissSavedViewThreshold(atom state.Atom[map[string]bool], key string) {
	cur := atom.Get()
	next := make(map[string]bool, len(cur)+1)
	for k, v := range cur {
		next[k] = v
	}
	next[key] = true
	atom.Set(next)
	persistSavedViewDismissals(next)
}

func loadSavedViewDismissals() map[string]bool {
	raw := kvGet(savedViewDismissKey)
	if raw == "" {
		return map[string]bool{}
	}
	var m map[string]bool
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]bool{}
	}
	return m
}

func persistSavedViewDismissals(m map[string]bool) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	kvSet(savedViewDismissKey, string(b))
}
