// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/syncstate"
)

func TestArtifactUploadRequiresDataProtection(t *testing.T) {
	oldLock := lsGet(appLockKey)
	oldStatus := lsGet(syncStatusKey)
	oldPasscode := activePasscode
	t.Cleanup(func() {
		if oldLock == "" {
			lsRemove(appLockKey)
		} else {
			lsSet(appLockKey, oldLock)
		}
		if oldStatus == "" {
			lsRemove(syncStatusKey)
		} else {
			lsSet(syncStatusKey, oldStatus)
		}
		activePasscode = oldPasscode
	})
	lsRemove(appLockKey)
	activePasscode = ""

	_, err := uploadBackendArtifactBlob(context.Background(), "http://invalid", "token", "workspace", domain.Artifact{Bytes: []byte("private")})
	if err == nil || !strings.Contains(err.Error(), syncstate.DataProtectionMessage) {
		t.Fatalf("upload error = %v, want protection status %q", err, syncstate.DataProtectionMessage)
	}
	var status syncStatus
	if err := json.Unmarshal([]byte(lsGet(syncStatusKey)), &status); err != nil {
		t.Fatalf("read persisted sync status: %v", err)
	}
	if status.State != string(syncstate.StatusNeedsDataProtection) || status.Message != syncstate.DataProtectionMessage {
		t.Fatalf("sync status = %+v, want typed data protection status", status)
	}
	if _, err := prepareBackendSyncDataset(context.Background(), "http://invalid", "token", "workspace", []byte(`{"artifacts":[]}`)); err == nil || !strings.Contains(err.Error(), syncstate.DataProtectionMessage) {
		t.Fatalf("snapshot prepare error = %v, want protection refusal", err)
	}
}
