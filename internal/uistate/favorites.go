// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/GoWebComponents/v5/state"
)

const (
	favoritesAtomID  = "nav:favorites"
	favoritesStore   = "cashflux:nav-favorites"
	favoritesSetFlag = "cashflux:nav-favorites-set"
)

// UseFavorites returns the shared atom holding the household's pinned rail
// destinations, in slot order, seeded from localStorage.
//
// The zero value is not "no favorites": on a browser that has never pinned
// anything the caller seeds this with the screens Alt+1..9 already went to, so
// the number keys keep doing what they did before pinning existed. Someone who
// has deliberately unpinned everything is a different state entirely, and
// FavoritesChosen distinguishes the two — without it, clearing the list would
// silently restore the defaults and the pins would look impossible to remove.
func UseFavorites() state.Atom[[]string] {
	return state.UseAtom(favoritesAtomID, loadFavorites())
}

// FavoritesChosen reports whether this browser has ever set the list, so an empty
// list can mean "none, deliberately" rather than "not yet seeded".
func FavoritesChosen() bool { return SettingKVGet(favoritesSetFlag) == "1" }

// PersistFavorites saves the pinned list and records that the household has made
// a choice, so an empty list stays empty across reloads.
func PersistFavorites(list []string) {
	data, err := json.Marshal(list)
	if err != nil {
		return
	}
	// SettingKVSetDeferred, not kvSet: a plain kvSet only reaches durable storage
	// on the 4-second autosave tick, so a reload within that window silently
	// restored the seeded order — measured, after reordering a pin and reloading
	// 1.2s later. The deferred settings write flushes on a 250ms debounce instead,
	// without paying a full dataset export per pin the way the leading edge would.
	SettingKVSetDeferred(favoritesStore, string(data))
	SettingKVSetDeferred(favoritesSetFlag, "1")
}

// loadFavorites reads the saved pins, or nil when absent or unreadable. nil means
// "seed from the defaults" only while FavoritesChosen is false.
func loadFavorites() []string {
	raw := SettingKVGet(favoritesStore)
	if raw == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	return list
}
