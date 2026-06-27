// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

// persistNow is the app-registered hook that forces an immediate dataset
// autosave, bypassing the periodic ticker. It is nil until the app wires it.
var persistNow func()

// CapturePersistNow registers the function that flushes the dataset to storage
// right away. The app package sets this to its resaveDataset closure during
// boot so screens (which can't reach the unexported autosave) can request an
// immediate persist after a bulk mutation.
func CapturePersistNow(f func()) { persistNow = f }

// RequestPersist forces the autosaved dataset to be written immediately, if the
// hook is wired. Call after an action that populates/changes a lot of data and
// might be followed quickly by a reload — e.g. loading the sample dataset (C2),
// where a reload within the autosave tick would otherwise race the write and
// lose the data. No-op before the app boots.
func RequestPersist() {
	if persistNow != nil {
		persistNow()
	}
}
