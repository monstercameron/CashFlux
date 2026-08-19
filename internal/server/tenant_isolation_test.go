// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"strings"
	"testing"
	"time"
)

// One adversarial sweep across every seam where one account could reach
// another's data.
//
// Workspace ids are minted by clients and collide by design — every install
// names its first workspace "default" — so "the other account's workspace" is
// not a hypothetical an attacker has to guess at. It is the SAME STRING this
// account already uses. Every check below therefore has both accounts holding a
// workspace called "default", which is the shape that actually occurs.
//
// The rule being enforced: nothing a caller supplies (workspace id, blob hash,
// snapshot version) may widen what they can see beyond their own account.

type tenants struct {
	store    *Store
	sync     *SyncService
	ctxA     context.Context
	ctxB     context.Context
	hashA    string
	now      time.Time
	root     string
	userA    string
	userB    string
	sharedWS string
}

func setupTenants(t *testing.T) *tenants {
	t.Helper()
	store := openTestStore(t)
	now := time.Now().UTC()
	tn := &tenants{
		store: store, sync: NewSyncService(store), now: now, root: t.TempDir(),
		userA: "user-A", userB: "user-B", sharedWS: "default",
	}
	tn.ctxA = ContextWithAuthUser(context.Background(), AuthUser{ID: tn.userA})
	tn.ctxB = ContextWithAuthUser(context.Background(), AuthUser{ID: tn.userB})
	seedSyncUser(t, store, tn.userA, now)
	seedSyncUser(t, store, tn.userB, now)

	// Both accounts hold a workspace with the SAME id.
	for _, tc := range []struct {
		ctx     context.Context
		user    string
		dataset string
	}{
		{tn.ctxA, tn.userA, `{"schemaVersion":1,"secret":"A-private","transactions":[{"id":"a1"}]}`},
		{tn.ctxB, tn.userB, `{"schemaVersion":1,"secret":"B-private","transactions":[{"id":"b1"}]}`},
	} {
		if _, err := tn.sync.PutWorkspace(tc.ctx, Workspace{ID: tn.sharedWS, Name: "Default"}, now, false, now); err != nil {
			t.Fatalf("seed workspace for %s: %v", tc.user, err)
		}
		if err := store.PutSnapshot(Snapshot{
			UserID: tc.user, WorkspaceID: tn.sharedWS, Dataset: []byte(tc.dataset), Version: 1, UpdatedAt: now,
		}, 0, 5); err != nil {
			t.Fatalf("seed snapshot for %s: %v", tc.user, err)
		}
	}

	// A private attachment belonging to A only.
	blob, err := store.PutBlob(tn.root, []byte("A's receipt"), "text/plain", "receipt.txt", 1<<20)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	tn.hashA = blob.Hash
	if err := store.LinkWorkspaceBlob(tn.userA, tn.sharedWS, tn.hashA); err != nil {
		t.Fatalf("LinkWorkspaceBlob: %v", err)
	}
	return tn
}

// B must not read A's snapshot through a workspace id they both use.
func TestTenantIsolationSnapshotReads(t *testing.T) {
	tn := setupTenants(t)

	snap, ok, err := tn.store.GetSnapshotForUser(tn.userB, tn.sharedWS)
	if err != nil {
		t.Fatalf("GetSnapshotForUser: %v", err)
	}
	if !ok {
		t.Fatal("B cannot read its OWN snapshot — the scoping went too far")
	}
	if strings.Contains(string(snap.Dataset), "A-private") {
		t.Fatal("B read A's snapshot")
	}
	if !strings.Contains(string(snap.Dataset), "B-private") {
		t.Fatalf("B got the wrong snapshot: %s", snap.Dataset)
	}
}

// The workspace record itself, and the list.
func TestTenantIsolationWorkspaceReads(t *testing.T) {
	tn := setupTenants(t)

	ws, ok, err := tn.sync.Get(tn.ctxB, tn.sharedWS)
	if err != nil || !ok {
		t.Fatalf("B cannot read its own workspace: ok=%v err=%v", ok, err)
	}
	if ws.UserID != tn.userB {
		t.Fatalf("Get returned a workspace owned by %q", ws.UserID)
	}
	list, err := tn.sync.List(tn.ctxB, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("B sees %d workspaces, want exactly its own", len(list))
	}
	for _, w := range list {
		if w.UserID != tn.userB {
			t.Fatalf("B's list contains a workspace owned by %q", w.UserID)
		}
	}
}

// Snapshot history is a read path too, and an older version of A's data is just
// as private as the current one.
func TestTenantIsolationSnapshotHistory(t *testing.T) {
	tn := setupTenants(t)
	// Give A a second version so there is history to reach for.
	if err := tn.store.PutSnapshot(Snapshot{
		UserID: tn.userA, WorkspaceID: tn.sharedWS, Dataset: []byte(`{"secret":"A-v2"}`), Version: 2, UpdatedAt: tn.now.Add(time.Minute),
	}, 0, 5); err != nil {
		t.Fatalf("seed A history: %v", err)
	}

	history, err := tn.store.SnapshotHistory(tn.userB, tn.sharedWS, 0)
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	for _, h := range history {
		if h.UserID != tn.userB || strings.Contains(string(h.Dataset), "A-") {
			t.Fatalf("B read A's snapshot history: %+v", h)
		}
	}
}

// Attachments are content-addressed and globally deduplicated, so knowing a hash
// must not be enough — the link has to belong to the caller.
func TestTenantIsolationBlobLinks(t *testing.T) {
	tn := setupTenants(t)

	linked, err := tn.store.UserWorkspaceBlob(tn.userA, tn.sharedWS, tn.hashA)
	if err != nil || !linked {
		t.Fatalf("owner cannot reach its own attachment: linked=%v err=%v", linked, err)
	}
	// B knows the hash and uses the same workspace id. Still no.
	linked, err = tn.store.UserWorkspaceBlob(tn.userB, tn.sharedWS, tn.hashA)
	if err != nil {
		t.Fatalf("UserWorkspaceBlob(B): %v", err)
	}
	if linked {
		t.Fatal("B can reach A's attachment through an identically-named workspace")
	}

	blobs, err := tn.store.WorkspaceBlobs(tn.userB, tn.sharedWS)
	if err != nil {
		t.Fatalf("WorkspaceBlobs: %v", err)
	}
	if len(blobs) != 0 {
		t.Fatalf("B's attachment list contains %d of A's blobs", len(blobs))
	}
}

// Storage accounting must not bill or reveal one account's data to another.
func TestTenantIsolationStorageAccounting(t *testing.T) {
	tn := setupTenants(t)
	usedB, err := tn.store.UserBlobBytes(tn.userB)
	if err != nil {
		t.Fatalf("UserBlobBytes(B): %v", err)
	}
	if usedB != 0 {
		t.Fatalf("B is charged %d bytes for A's attachment", usedB)
	}
	usedA, err := tn.store.UserBlobBytes(tn.userA)
	if err != nil {
		t.Fatalf("UserBlobBytes(A): %v", err)
	}
	if usedA == 0 {
		t.Fatal("A is not charged for its own attachment — the scoping went too far")
	}
}

// Writes are a leak surface too: B must not be able to modify or delete A's
// workspace by naming it.
func TestTenantIsolationWrites(t *testing.T) {
	tn := setupTenants(t)

	// B writing "default" writes its OWN, never A's.
	if _, err := tn.sync.PutWorkspace(tn.ctxB, Workspace{ID: tn.sharedWS, Name: "B renamed it"}, tn.now.Add(time.Hour), true, tn.now.Add(time.Hour)); err != nil {
		t.Fatalf("B's own write refused: %v", err)
	}
	aws, ok, err := tn.store.GetWorkspace(tn.userA, tn.sharedWS)
	if err != nil || !ok {
		t.Fatalf("A's workspace: ok=%v err=%v", ok, err)
	}
	if aws.Name != "Default" {
		t.Fatalf("B renamed A's workspace to %q", aws.Name)
	}

	// And B deleting theirs leaves A's alone.
	if _, err := tn.sync.Delete(tn.ctxB, tn.sharedWS, tn.now.Add(2*time.Hour), "dev"); err != nil {
		t.Fatalf("B delete: %v", err)
	}
	aws, ok, _ = tn.store.GetWorkspace(tn.userA, tn.sharedWS)
	if !ok || aws.Deleted {
		t.Fatal("B's delete tombstoned A's workspace")
	}
	asnap, ok, _ := tn.store.GetSnapshotForUser(tn.userA, tn.sharedWS)
	if !ok || !strings.Contains(string(asnap.Dataset), "A-private") {
		t.Fatal("B's delete destroyed A's snapshot")
	}
}

// The account export is the bulk read path — the one place a scoping mistake
// hands over everything at once.
func TestTenantIsolationAccountExport(t *testing.T) {
	tn := setupTenants(t)
	export, ok, err := tn.store.ExportAccount(tn.userB, tn.now)
	if err != nil || !ok {
		t.Fatalf("ExportAccount(B): ok=%v err=%v", ok, err)
	}
	for _, w := range export.Workspaces {
		if w.UserID != tn.userA {
			continue
		}
		t.Fatalf("B's export contains A's workspace: %+v", w)
	}
	for _, s := range export.Snapshots {
		if strings.Contains(string(s.Dataset), "A-private") {
			t.Fatal("B's export contains A's snapshot data")
		}
	}
	for _, b := range export.Blobs {
		if b.Hash == tn.hashA {
			t.Fatal("B's export lists A's attachment")
		}
	}
}

// Deleting one account must not touch the other's identically-named workspace,
// snapshot, history or attachment links.
func TestTenantIsolationAccountDeletion(t *testing.T) {
	tn := setupTenants(t)
	if _, err := tn.store.DeleteAccount(tn.userA); err != nil {
		t.Fatalf("DeleteAccount(A): %v", err)
	}
	if _, ok, _ := tn.store.GetSnapshotForUser(tn.userB, tn.sharedWS); !ok {
		t.Fatal("deleting A destroyed B's snapshot")
	}
	list, err := tn.store.ListWorkspaces(tn.userB, true)
	if err != nil || len(list) != 1 {
		t.Fatalf("B owns %d workspaces after A was deleted (err %v), want 1", len(list), err)
	}
	// And A's rows really are gone.
	if _, ok, _ := tn.store.GetSnapshotForUser(tn.userA, tn.sharedWS); ok {
		t.Fatal("A's snapshot survived the account deletion")
	}
}
