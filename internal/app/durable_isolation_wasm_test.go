// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "testing"

// isolateDurableGlobals clears the package-level persistence flags that an earlier
// test can leave set: the autosave and sync pauses, the quarantine state, the
// pending envelope and the hosted hydration gate. Each flag is restored when the
// test ends, so no test depends on the one that ran before it.
func isolateDurableGlobals(t *testing.T) {
	t.Helper()
	oldSuspend, oldSync, oldPending := suspendAutosave, syncApplyPaused, pendingEnvelopeRaw
	oldQuarantined, oldQuarantinePending := datasetQuarantined, quarantinePending
	oldRaw, oldAvailable := quarantineRaw, quarantineAvailable
	oldHosted := currentHostedHydration()

	suspendAutosave, syncApplyPaused, pendingEnvelopeRaw = false, false, ""
	datasetQuarantined, quarantinePending = false, false
	quarantineRaw, quarantineAvailable = "", false
	setHostedHydration(hostedHydrationStatus{})

	t.Cleanup(func() {
		suspendAutosave, syncApplyPaused, pendingEnvelopeRaw = oldSuspend, oldSync, oldPending
		datasetQuarantined, quarantinePending = oldQuarantined, oldQuarantinePending
		quarantineRaw, quarantineAvailable = oldRaw, oldAvailable
		setHostedHydration(oldHosted)
	})
}
