// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/GoWebComponents/v4/state"
)

// sampleActiveKey flags that the loaded dataset is the seeded demo (L6). It lives in
// the dataset's app KV (the single source of truth) — a property of the dataset, so
// it travels with export/import and is cleared by a wipe (a wiped dataset is not the
// sample, so the "you're viewing sample data" banner correctly disappears). It is set
// when the sample is seeded and cleared when the user personalizes, imports, wipes,
// or dismisses the banner. (Distinct from cashflux:seeded, the browser-store bootstrap
// gate that decides whether to seed at all — that one can't live in the dataset.)
const sampleActiveKey = "cashflux:sampleActive"

// UseSampleActive returns the shared atom that tracks whether the app is
// currently showing the seeded sample dataset. True = banner should be shown;
// false = user has personalized or dismissed. The atom is initialised from the
// dataset app KV so the first render reflects the persisted state.
func UseSampleActive() state.Atom[bool] {
	initial := KVGet(sampleActiveKey) == "1"
	return state.UseAtom("app:sampleActive", initial)
}

// SetSampleActive writes the dataset app-KV flag and updates the atom so any
// subscribed component re-renders immediately. Call with true when the sample
// is seeded; call with false on wipe, import, or banner dismiss.
func SetSampleActive(v bool) {
	if v {
		KVSet(sampleActiveKey, "1")
	} else {
		KVDelete(sampleActiveKey)
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
