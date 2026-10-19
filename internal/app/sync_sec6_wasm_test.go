// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/syncstate"
)

func TestUnencryptedSyncQueueStaysQueuedWithProtectionStatus(t *testing.T) {
	oldQueue := lsGet(syncQueueKey)
	oldStatus := lsGet(syncStatusKey)
	oldPasscode := activePasscode
	t.Cleanup(func() {
		if oldQueue == "" {
			lsRemove(syncQueueKey)
		} else {
			lsSet(syncQueueKey, oldQueue)
		}
		loadSyncQueue() // refresh the in-memory pending count after restoring storage
		if oldStatus == "" {
			lsRemove(syncStatusKey)
		} else {
			lsSet(syncStatusKey, oldStatus)
		}
		activePasscode = oldPasscode
	})

	activePasscode = ""
	queued := []queuedSyncMutation{{WorkspaceID: "sec6-workspace", Hash: "sec6-hash", Dataset: `{"secret":"local"}`}}
	saveSyncQueue(queued)
	if requireDatasetEncryption() {
		t.Fatal("sync allowed a dataset without passcode encryption")
	}
	gotQueue := loadSyncQueue()
	if len(gotQueue) != 1 || gotQueue[0].Hash != queued[0].Hash || gotQueue[0].Dataset != queued[0].Dataset {
		t.Fatalf("unencrypted queue changed or was dropped: %#v", gotQueue)
	}
	var status syncStatus
	if err := json.Unmarshal([]byte(lsGet(syncStatusKey)), &status); err != nil {
		t.Fatalf("read persisted sync status: %v", err)
	}
	if status.State != string(syncstate.StatusNeedsDataProtection) || status.Message != syncstate.DataProtectionMessage || status.Pending != 1 {
		t.Fatalf("sync status = %+v, want typed protection status with one queued item", status)
	}
}
