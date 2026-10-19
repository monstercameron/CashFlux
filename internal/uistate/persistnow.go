// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"time"

	"github.com/monstercameron/GoWebComponents/v5/state"
)

var persistFailures = map[string]func(){}
var persistFailureVersion int
var persistFailureAtom state.Atom[int]
var persistFailureCaptured bool
var persistRecoveryNext uint64
var persistRecoveryObservers = map[uint64]func(){}

// OnPersistRecovered subscribes to successful recovery so deferred feedback can
// also complete when autosave, rather than the Retry button, saves the changes.
func OnPersistRecovered(recovered func()) func() {
	persistRecoveryNext++
	id := persistRecoveryNext
	persistRecoveryObservers[id] = recovered
	return func() { delete(persistRecoveryObservers, id) }
}

// UsePersistFailures subscribes to outstanding durable-storage failures. Failures
// stay visible until their own writer succeeds; a toast timeout cannot clear them.
func UsePersistFailures() bool {
	a := state.UseAtom("app:unsaved-changes", persistFailureVersion)
	persistFailureAtom, persistFailureCaptured = a, true
	return len(persistFailures) > 0
}

// ReportPersistFailure retains a retry for one failed writer without discarding
// failures from other writers (for example preferences and household records).
func ReportPersistFailure(key string, retry func()) {
	persistFailures[key] = retry
	refreshPersistFailures()
}

// ClearPersistFailure clears only the writer whose durable commit succeeded.
func ClearPersistFailure(key string) {
	if _, ok := persistFailures[key]; ok {
		delete(persistFailures, key)
		refreshPersistFailures()
		for _, recovered := range persistRecoveryObservers {
			// Recovery callbacks may request another save. Leave the committing
			// writer's callback before allowing that reentrancy.
			time.AfterFunc(time.Millisecond, recovered)
		}
	}
}

func refreshPersistFailures() {
	persistFailureVersion++
	if persistFailureCaptured {
		persistFailureAtom.Set(persistFailureVersion)
	}
}

// RetryPersistFailures retries a stable copy: a synchronous completion can remove
// its entry, and one failure must not prevent another writer from being retried.
func RetryPersistFailures() {
	// Mark failed preference writes as queued before retrying dependent notices.
	// Otherwise map order could make a notice see the old error and require a
	// second click even though the preference's first retry is about to succeed.
	retryChromePreferences()
	retries := make([]func(), 0, len(persistFailures))
	for _, retry := range persistFailures {
		if retry != nil {
			retries = append(retries, retry)
		}
	}
	for _, retry := range retries {
		retry()
	}
}

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
