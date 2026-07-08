// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strconv"

	"github.com/monstercameron/GoWebComponents/v4/state"
)

const (
	muzakAtomID  = "app:muzak"
	muzakStoreID = "cashflux:muzak"

	muzakVolAtomID  = "app:muzak-volume"
	muzakVolStoreID = "cashflux:muzak-volume"

	// DefaultMuzakVolume is the low starting volume (0..1) for the background music.
	DefaultMuzakVolume = 0.12
)

// UseMuzakEnabled returns the shared on/off atom for the background music, seeded
// from localStorage. Defaults to ON (music plays at low volume until toggled off).
// The atom is captured so SetMuzakEnabled can drive it from outside a render.
func UseMuzakEnabled() state.Atom[bool] {
	a := state.UseAtom(muzakAtomID, loadMuzakEnabled())
	capturedMuzakEnabled = a
	muzakEnabledCaptured = true
	return a
}

var (
	capturedMuzakEnabled state.Atom[bool]
	muzakEnabledCaptured bool
)

// SetMuzakEnabled sets the music on/off from OUTSIDE a component render — used by
// the raw-JS app-lock gate's mute button. It updates the shared atom (so the
// top-bar toggle and the player-driving effect stay in sync — otherwise the effect
// would re-apply the stale value and the music would resume) AND persists the
// choice. The persist always happens; the atom update is a no-op until a component
// has read the atom once (which the always-rendered top bar does).
func SetMuzakEnabled(on bool) {
	PersistMuzakEnabled(on)
	if muzakEnabledCaptured {
		capturedMuzakEnabled.Set(on)
	}
}

// PersistMuzakEnabled remembers the music on/off choice across reloads.
func PersistMuzakEnabled(on bool) {
	v := "1"
	if !on {
		v = "0"
	}
	SettingKVSet(muzakStoreID, v)
}

// loadMuzakEnabled reads the saved choice, defaulting to ON when unset.
func loadMuzakEnabled() bool {
	raw := SettingKVGet(muzakStoreID)
	if raw == "" {
		return true
	}
	return raw != "0"
}

// UseMuzakVolume returns the shared background-music volume atom (0..1), seeded
// from localStorage. Defaults to DefaultMuzakVolume (low).
func UseMuzakVolume() state.Atom[float64] {
	return state.UseAtom(muzakVolAtomID, loadMuzakVolume())
}

// PersistMuzakVolume remembers the music volume across reloads.
func PersistMuzakVolume(v float64) {
	v = clampVolume(v)
	SettingKVSet(muzakVolStoreID, strconv.FormatFloat(v, 'f', 3, 64))
}

// loadMuzakVolume reads the saved volume, defaulting to DefaultMuzakVolume.
func loadMuzakVolume() float64 {
	raw := SettingKVGet(muzakVolStoreID)
	if raw == "" {
		return DefaultMuzakVolume
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return DefaultMuzakVolume
	}
	return clampVolume(f)
}

func clampVolume(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
