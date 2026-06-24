// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/GoWebComponents/state"
)

// sampleActiveKey is the localStorage flag written when the sample dataset is
// seeded on a genuine first run (L6). It is cleared when the user dismisses the
// "you're viewing sample data" banner or wipes / imports their own data.
const sampleActiveKey = "cashflux:sampleActive"

// UseSampleActive returns the shared atom that tracks whether the app is
// currently showing the seeded sample dataset. True = banner should be shown;
// false = user has personalized or dismissed. The atom is initialised from
// localStorage so the first render reflects the persisted state.
func UseSampleActive() state.Atom[bool] {
	initial := browserstore.GetString(sampleActiveKey) == "1"
	return state.UseAtom("app:sampleActive", initial)
}

// SetSampleActive writes the localStorage flag and updates the atom so any
// subscribed component re-renders immediately. Call with true when the sample
// is seeded; call with false on wipe, import, or banner dismiss.
func SetSampleActive(v bool) {
	if v {
		browserstore.Set(sampleActiveKey, "1")
	} else {
		browserstore.Remove(sampleActiveKey)
	}
	// Notify via the captured atom if the banner has already mounted.
	if sampleActiveReady {
		sampleActiveAtom.Set(v)
	}
}

var (
	sampleActiveAtom  state.Atom[bool]
	sampleActiveReady bool
)

// CaptureSampleActive registers the atom the SampleDataBanner renders with, so
// SetSampleActive can post a state change from outside the component tree (e.g.
// from persist.go's hydrate path or from wipeData in settings.go).
func CaptureSampleActive(a state.Atom[bool]) {
	sampleActiveAtom, sampleActiveReady = a, true
}
