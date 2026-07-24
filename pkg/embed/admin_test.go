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

// TestAdminListUsersIncludesMonthlyRequestVolume proves each listed user
// carries its own current-month request total, not another user's or a
// running lifetime total.
func TestAdminListUsersIncludesMonthlyRequestVolume(t *testing.T) {
	a := newTestAdmin(t)
	now := time.Now().UTC()

	if err := a.store.UpsertUser(server.User{ID: "u1", Provider: "device", Subject: "u1", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser u1: %v", err)
	}
	if err := a.store.UpsertUser(server.User{ID: "u2", Provider: "device", Subject: "u2", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser u2: %v", err)
	}
	if _, err := a.store.AddUsage("u1", now, 7, 100); err != nil {
		t.Fatalf("AddUsage u1 today: %v", err)
	}
	if _, err := a.store.AddUsage("u1", now.AddDate(0, -2, 0), 40, 400); err != nil {
		t.Fatalf("AddUsage u1 two months ago: %v", err)
	}
	if _, err := a.store.AddUsage("u2", now, 3, 30); err != nil {
		t.Fatalf("AddUsage u2 today: %v", err)
	}

	users, err := a.ListUsers(50, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	byID := map[string]User{}
	for _, u := range users {
		byID[u.ID] = u
	}
	if got := byID["u1"].RequestsThisMonth; got != 7 {
		t.Fatalf("u1 RequestsThisMonth = %d, want 7 (the two-months-ago row must not count)", got)
	}
	if got := byID["u2"].RequestsThisMonth; got != 3 {
		t.Fatalf("u2 RequestsThisMonth = %d, want 3", got)
	}
}

func TestAdminListUsersEmpty(t *testing.T) {
	a := newTestAdmin(t)
	users, err := a.ListUsers(50, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("ListUsers on a fresh store: got %d users, want 0", len(users))
	}
}

// TestAdminStorageStats proves the reported sizes are real numbers, not
// zero-valued placeholders.
func TestAdminStorageStats(t *testing.T) {
	a := newTestAdmin(t)
	dbBytes, blobBytes, err := a.StorageStats()
	if err != nil {
		t.Fatalf("StorageStats: %v", err)
	}
	if dbBytes <= 0 {
		t.Fatalf("StorageStats: db size = %d, want > 0", dbBytes)
	}
	if blobBytes != 0 {
		t.Fatalf("StorageStats: blob size = %d, want 0 on a fresh store with no blobs", blobBytes)
	}
}

// TestAdminMintActivationCodeBindsOneOwnerAccount proves the whole point of
// the activation-code flow: every code resolves to the SAME account, so a
// second activated device joins the first one's data instead of landing in an
// island of its own. It also proves the account is created lazily on the
// first mint (a fresh store has no users at all).
func TestAdminMintActivationCodeBindsOneOwnerAccount(t *testing.T) {
	a := newTestAdmin(t)

	first, expiresAt, err := a.MintActivationCode()
	if err != nil {
		t.Fatalf("MintActivationCode: %v", err)
	}
	if first == "" {
		t.Fatal("MintActivationCode: expected a non-empty code")
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("MintActivationCode: expiresAt = %s, want a future time", expiresAt)
	}

	second, _, err := a.MintActivationCode()
	if err != nil {
		t.Fatalf("MintActivationCode (second): %v", err)
	}
	if second == first {
		t.Fatal("MintActivationCode: expected a distinct code per mint")
	}

	users, err := a.ListUsers(50, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 || users[0].ID != OwnerAccountID {
		t.Fatalf("ListUsers after two mints: got %+v, want exactly one %s", users, OwnerAccountID)
	}

	now := time.Now().UTC()
	for _, code := range []string{first, second} {
		userID, ok, err := a.store.ConsumePairingCode(code, now)
		if err != nil {
			t.Fatalf("ConsumePairingCode(%s): %v", code, err)
		}
		if !ok {
			t.Fatalf("ConsumePairingCode(%s): expected a redeemable code", code)
		}
		if userID != OwnerAccountID {
			t.Fatalf("ConsumePairingCode(%s): user = %q, want %q", code, userID, OwnerAccountID)
		}
	}
}

// TestAdminMintActivationCodeSingleUse proves a code cannot be replayed — the
// second redemption of the same code always fails, so a code shoulder-surfed
// after Cam has already used it is worthless.
func TestAdminMintActivationCodeSingleUse(t *testing.T) {
	a := newTestAdmin(t)
	code, _, err := a.MintActivationCode()
	if err != nil {
		t.Fatalf("MintActivationCode: %v", err)
	}
	now := time.Now().UTC()
	if _, ok, err := a.store.ConsumePairingCode(code, now); err != nil || !ok {
		t.Fatalf("first ConsumePairingCode: ok=%v err=%v", ok, err)
	}
	if _, ok, err := a.store.ConsumePairingCode(code, now); err != nil || ok {
		t.Fatalf("second ConsumePairingCode: ok=%v err=%v, want ok=false", ok, err)
	}
}

// TestAdminMintActivationCodeUnconfigured proves an unconfigured Admin (no
// CashFlux embedding on this deployment) reports an error rather than
// panicking on a nil store.
func TestAdminMintActivationCodeUnconfigured(t *testing.T) {
	var a *Admin
	if _, _, err := a.MintActivationCode(); err == nil {
		t.Fatal("MintActivationCode on a nil Admin: expected an error")
	}
	if _, _, err := (&Admin{}).MintActivationCode(); err == nil {
		t.Fatal("MintActivationCode with no store: expected an error")
	}
}
