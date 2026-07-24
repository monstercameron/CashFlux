// SPDX-License-Identifier: MIT

package embed

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/server"
)

func newTestAdmin(t *testing.T) *Admin {
	t.Helper()
	store, err := server.OpenStore(filepath.Join(t.TempDir(), "cashflux.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &Admin{store: store}
}

func TestAdminListPendingDevices(t *testing.T) {
	a := newTestAdmin(t)
	now := time.Now().UTC()
	if _, _, err := a.store.MintPendingDevice("kitchen-tablet", now); err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	list, err := a.ListPendingDevices()
	if err != nil {
		t.Fatalf("ListPendingDevices: %v", err)
	}
	if len(list) != 1 || list[0].Label != "kitchen-tablet" {
		t.Fatalf("ListPendingDevices: got %+v", list)
	}
}

// TestAdminApprovePairingCreatesDistinctAccount proves each approval mints a
// BRAND-NEW account (TODOS.md C454) — this deployment admits distinct
// people/devices, not one shared identity — and the code it returns is the
// SAME code the store attached to the pending_devices row (what
// WatchPairingStatus pushes to the waiting device), so admin-console and
// device displays genuinely match for the human cross-check.
func TestAdminApprovePairingCreatesDistinctAccount(t *testing.T) {
	a := newTestAdmin(t)
	now := time.Now().UTC()
	deviceID, _, err := a.store.MintPendingDevice("phone", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	approved, code, err := a.ApprovePairing(deviceID)
	if err != nil {
		t.Fatalf("ApprovePairing: %v", err)
	}
	if !approved {
		t.Fatal("ApprovePairing: expected approved=true")
	}
	if code == "" {
		t.Fatal("ApprovePairing: expected a non-empty pairing code")
	}
	pd, ok, err := a.store.GetPendingDevice(deviceID)
	if err != nil || !ok {
		t.Fatalf("GetPendingDevice: ok=%v err=%v", ok, err)
	}
	if pd.PairingCode != code {
		t.Fatalf("ApprovePairing returned code %q, but the pending_devices row (what WatchPairingStatus pushes) carries %q", code, pd.PairingCode)
	}

	// A second, independent approval must mint a DIFFERENT account.
	deviceID2, _, err := a.store.MintPendingDevice("laptop", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	_, code2, err := a.ApprovePairing(deviceID2)
	if err != nil {
		t.Fatalf("second ApprovePairing: %v", err)
	}
	userID1, ok1, err := a.store.PeekPairingCodeUserID(code)
	if err != nil || !ok1 {
		t.Fatalf("PeekPairingCodeUserID(1): ok=%v err=%v", ok1, err)
	}
	userID2, ok2, err := a.store.PeekPairingCodeUserID(code2)
	if err != nil || !ok2 {
		t.Fatalf("PeekPairingCodeUserID(2): ok=%v err=%v", ok2, err)
	}
	if userID1 == userID2 {
		t.Fatalf("ApprovePairing: two independent approvals minted codes for the SAME account %q — expected distinct accounts", userID1)
	}
}

// TestAdminApprovePairingOneShot proves a second approval attempt on an
// already-resolved request reports approved=false rather than silently
// minting and discarding a second, orphaned account.
func TestAdminApprovePairingOneShot(t *testing.T) {
	a := newTestAdmin(t)
	now := time.Now().UTC()
	deviceID, _, err := a.store.MintPendingDevice("phone", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	if approved, _, err := a.ApprovePairing(deviceID); err != nil || !approved {
		t.Fatalf("first ApprovePairing: approved=%v err=%v", approved, err)
	}
	approved, code, err := a.ApprovePairing(deviceID)
	if err != nil {
		t.Fatalf("second ApprovePairing: %v", err)
	}
	if approved || code != "" {
		t.Fatalf("second ApprovePairing: approved=%v code=%q, want false/empty for an already-resolved request", approved, code)
	}
}

func TestAdminApprovePairingUnknownDevice(t *testing.T) {
	a := newTestAdmin(t)
	approved, code, err := a.ApprovePairing("never-minted")
	if err != nil {
		t.Fatalf("ApprovePairing: %v", err)
	}
	if approved || code != "" {
		t.Fatalf("ApprovePairing for an unknown device: approved=%v code=%q, want false/empty", approved, code)
	}
}

func TestAdminRejectPairing(t *testing.T) {
	a := newTestAdmin(t)
	now := time.Now().UTC()
	deviceID, _, err := a.store.MintPendingDevice("phone", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	rejected, err := a.RejectPairing(deviceID)
	if err != nil {
		t.Fatalf("RejectPairing: %v", err)
	}
	if !rejected {
		t.Fatal("RejectPairing: expected rejected=true")
	}
	// One-shot: rejecting again reports nothing to reject.
	rejected, err = a.RejectPairing(deviceID)
	if err != nil {
		t.Fatalf("second RejectPairing: %v", err)
	}
	if rejected {
		t.Fatal("RejectPairing: expected rejected=false for an already-resolved request")
	}
	// A rejected request can no longer be approved.
	approved, _, err := a.ApprovePairing(deviceID)
	if err != nil {
		t.Fatalf("ApprovePairing after reject: %v", err)
	}
	if approved {
		t.Fatal("ApprovePairing: expected approval of an already-rejected request to fail")
	}
}
