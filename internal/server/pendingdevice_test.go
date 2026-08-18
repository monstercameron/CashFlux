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

// TestExpiredRequestsBecomeReadableHistoryNotDeletions proves the C700 change
// of policy: a request that times out is MARKED expired rather than deleted, so
// an operator can still see that it happened, who it was for, and that nobody
// answered it. Deleting on expiry is what made a device account with no request
// behind it unexplainable in the console.
func TestExpiredRequestsBecomeReadableHistoryNotDeletions(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	past := now.Add(-2 * PendingDeviceTTL)

	// Mint the fresh row FIRST: MintPendingDevice's own opportunistic sweep
	// would otherwise run before this test's explicit call.
	freshID, _, err := store.MintPendingDevice("still-fresh", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	staleID, _, err := store.MintPendingDevice("expired-pending", past)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}

	expired, err := store.ExpirePendingDevices(now)
	if err != nil {
		t.Fatalf("ExpirePendingDevices: %v", err)
	}
	if expired != 1 {
		t.Fatalf("ExpirePendingDevices: marked %d rows, want 1", expired)
	}
	life, ok, err := store.GetDeviceLifecycle(staleID)
	if err != nil || !ok {
		t.Fatalf("GetDeviceLifecycle(stale): ok=%v err=%v - the row must survive to be read", ok, err)
	}
	if life.Status != PendingDeviceStatusExpired {
		t.Fatalf("stale status = %q, want %q", life.Status, PendingDeviceStatusExpired)
	}
	if life.ResolvedBy != ResolvedBySystem {
		t.Fatalf("stale resolvedBy = %q, want %q - the clock decided, not an operator", life.ResolvedBy, ResolvedBySystem)
	}
	if life.ResolvedAt.IsZero() {
		t.Fatal("an expired request must record WHEN it lapsed")
	}
	// The still-live request is untouched and still actionable.
	if pending, err := store.ListPendingDevices(now); err != nil || len(pending) != 1 || pending[0].DeviceID != freshID {
		t.Fatalf("ListPendingDevices = %+v (err %v), want only the fresh request", pending, err)
	}
}

// TestRetentionStillBoundsTheTable proves the security property the old
// delete-on-expiry policy existed for survives the change: rows are deleted,
// just later. Without this, keeping history would reintroduce the unbounded
// growth that devicePairingGlobalLimiter cannot prevent (it caps the mint RATE,
// never the cumulative total).
func TestRetentionStillBoundsTheTable(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	recent := now.Add(-2 * PendingDeviceTTL)
	ancient := now.Add(-PendingDeviceRetention - 48*time.Hour)

	recentID, _, err := store.MintPendingDevice("recently-expired", recent)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	ancientID, _, err := store.MintPendingDevice("long-expired", ancient)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}

	deleted, err := store.PruneExpiredPendingDevices(now)
	if err != nil {
		t.Fatalf("PruneExpiredPendingDevices: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted %d rows, want 1 - only the row past its retention window", deleted)
	}
	if _, ok, err := store.GetPendingDevice(ancientID); err != nil || ok {
		t.Fatalf("GetPendingDevice(ancient): ok=%v err=%v, want ok=false - retention must bound the table", ok, err)
	}
	if _, ok, err := store.GetPendingDevice(recentID); err != nil || !ok {
		t.Fatalf("GetPendingDevice(recent): ok=%v err=%v, want ok=true - recent history is what an operator reads", ok, err)
	}
}

// TestResolutionRecordsWhoDecided proves an admin refusal and a device
// withdrawal are distinguishable, which they were not when both wrote
// "rejected" through one code path.
func TestResolutionRecordsWhoDecided(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	rejectedID, _, err := store.MintPendingDevice("refused-by-operator", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	canceledID, _, err := store.MintPendingDevice("withdrawn-by-user", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	if ok, err := store.RejectPendingDevice(rejectedID); err != nil || !ok {
		t.Fatalf("RejectPendingDevice: ok=%v err=%v", ok, err)
	}
	if ok, err := store.CancelPendingDevice(canceledID); err != nil || !ok {
		t.Fatalf("CancelPendingDevice: ok=%v err=%v", ok, err)
	}
	for _, tc := range []struct{ id, status, by string }{
		{rejectedID, PendingDeviceStatusRejected, ResolvedByAdmin},
		{canceledID, PendingDeviceStatusCanceled, ResolvedByDevice},
	} {
		life, ok, err := store.GetDeviceLifecycle(tc.id)
		if err != nil || !ok {
			t.Fatalf("GetDeviceLifecycle(%s): ok=%v err=%v", tc.id, ok, err)
		}
		if life.Status != tc.status || life.ResolvedBy != tc.by {
			t.Errorf("%s resolved as (%q, %q), want (%q, %q)", tc.id, life.Status, life.ResolvedBy, tc.status, tc.by)
		}
	}
	// Neither can be resolved twice.
	if ok, _ := store.RejectPendingDevice(rejectedID); ok {
		t.Error("a resolved request was resolved again")
	}
}

// TestMintPendingDeviceSweepsRetiredRows proves the opportunistic sweep still
// runs on every real mint, so the table stays bounded without a scheduler.
func TestMintPendingDeviceSweepsRetiredRows(t *testing.T) {
	store := openTestStore(t)
	ancient := time.Now().UTC().Add(-PendingDeviceRetention - 48*time.Hour)
	staleID, _, err := store.MintPendingDevice("stale-device", ancient)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	if _, _, err := store.MintPendingDevice("fresh-device", time.Now().UTC()); err != nil {
		t.Fatalf("second MintPendingDevice: %v", err)
	}
	if _, ok, err := store.GetPendingDevice(staleID); err != nil || ok {
		t.Fatalf("GetPendingDevice(stale) after a later mint: ok=%v err=%v, want ok=false (swept)", ok, err)
	}
}
