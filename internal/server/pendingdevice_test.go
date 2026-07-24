// SPDX-License-Identifier: MIT

package server

import (
	"testing"
	"time"
)

func TestMintPendingDeviceHappyPath(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	id, expiresAt, err := store.MintPendingDevice("my-laptop", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	if id == "" {
		t.Fatal("MintPendingDevice: expected a non-empty device id")
	}
	if !expiresAt.Equal(now.Add(PendingDeviceTTL)) {
		t.Fatalf("MintPendingDevice: expiresAt = %v, want %v", expiresAt, now.Add(PendingDeviceTTL))
	}
	pd, ok, err := store.GetPendingDevice(id)
	if err != nil {
		t.Fatalf("GetPendingDevice: %v", err)
	}
	if !ok {
		t.Fatal("GetPendingDevice: expected the just-minted device to be found")
	}
	if pd.Status != PendingDeviceStatusPending {
		t.Fatalf("GetPendingDevice: status = %q, want %q", pd.Status, PendingDeviceStatusPending)
	}
	if pd.Label != "my-laptop" {
		t.Fatalf("GetPendingDevice: label = %q, want %q", pd.Label, "my-laptop")
	}
}

func TestGetPendingDeviceUnknownID(t *testing.T) {
	store := openTestStore(t)
	_, ok, err := store.GetPendingDevice("never-minted")
	if err != nil {
		t.Fatalf("GetPendingDevice: %v", err)
	}
	if ok {
		t.Fatal("GetPendingDevice: expected ok=false for an id that was never minted")
	}
}

func TestListPendingDevicesOnlyPendingUnexpired(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()

	pendingID, _, err := store.MintPendingDevice("pending-device", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	expiredID, _, err := store.MintPendingDevice("expired-device", now.Add(-2*PendingDeviceTTL))
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	approvedID, _, err := store.MintPendingDevice("approved-device", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	if ok, err := store.ApprovePendingDevice(approvedID, "123456", now); err != nil || !ok {
		t.Fatalf("ApprovePendingDevice: ok=%v err=%v", ok, err)
	}

	list, err := store.ListPendingDevices(now)
	if err != nil {
		t.Fatalf("ListPendingDevices: %v", err)
	}
	if len(list) != 1 || list[0].DeviceID != pendingID {
		t.Fatalf("ListPendingDevices: got %+v, want exactly [%s]", list, pendingID)
	}
	_ = expiredID
}

func TestApprovePendingDeviceHappyPathAndOneShot(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	id, _, err := store.MintPendingDevice("my-laptop", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	ok, err := store.ApprovePendingDevice(id, "654321", now)
	if err != nil {
		t.Fatalf("ApprovePendingDevice: %v", err)
	}
	if !ok {
		t.Fatal("ApprovePendingDevice: expected the first approval to succeed")
	}
	pd, found, err := store.GetPendingDevice(id)
	if err != nil || !found {
		t.Fatalf("GetPendingDevice: found=%v err=%v", found, err)
	}
	if pd.Status != PendingDeviceStatusApproved || pd.PairingCode != "654321" {
		t.Fatalf("GetPendingDevice after approve: got %+v", pd)
	}

	// One-shot: a second approval attempt on an already-resolved request must
	// not silently overwrite the decision (e.g. with a different pairing code).
	ok, err = store.ApprovePendingDevice(id, "999999", now)
	if err != nil {
		t.Fatalf("second ApprovePendingDevice: %v", err)
	}
	if ok {
		t.Fatal("ApprovePendingDevice: expected the second approval to fail (already resolved)")
	}
	pd, _, _ = store.GetPendingDevice(id)
	if pd.PairingCode != "654321" {
		t.Fatalf("ApprovePendingDevice: pairing code changed on re-approval, got %q, want unchanged %q", pd.PairingCode, "654321")
	}
}

func TestApprovePendingDeviceRejectsExpired(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	id, _, err := store.MintPendingDevice("my-laptop", now.Add(-2*PendingDeviceTTL))
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	ok, err := store.ApprovePendingDevice(id, "111111", now)
	if err != nil {
		t.Fatalf("ApprovePendingDevice: %v", err)
	}
	if ok {
		t.Fatal("ApprovePendingDevice: expected approval of an expired request to fail")
	}
}

func TestRejectPendingDeviceHappyPathAndOneShot(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	id, _, err := store.MintPendingDevice("my-laptop", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	ok, err := store.RejectPendingDevice(id)
	if err != nil {
		t.Fatalf("RejectPendingDevice: %v", err)
	}
	if !ok {
		t.Fatal("RejectPendingDevice: expected the first rejection to succeed")
	}
	pd, found, err := store.GetPendingDevice(id)
	if err != nil || !found {
		t.Fatalf("GetPendingDevice: found=%v err=%v", found, err)
	}
	if pd.Status != PendingDeviceStatusRejected {
		t.Fatalf("GetPendingDevice after reject: status = %q, want %q", pd.Status, PendingDeviceStatusRejected)
	}

	// A rejected request cannot later be approved — the decision is final.
	ok, err = store.ApprovePendingDevice(id, "222222", now)
	if err != nil {
		t.Fatalf("ApprovePendingDevice after reject: %v", err)
	}
	if ok {
		t.Fatal("ApprovePendingDevice: expected approval of an already-rejected request to fail")
	}
}

func TestRejectPendingDeviceUnknownID(t *testing.T) {
	store := openTestStore(t)
	ok, err := store.RejectPendingDevice("never-minted")
	if err != nil {
		t.Fatalf("RejectPendingDevice: %v", err)
	}
	if ok {
		t.Fatal("RejectPendingDevice: expected ok=false for an id that was never minted")
	}
}
