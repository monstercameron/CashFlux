// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"testing"
	"time"
)

// Workspace ids are minted by the client, and every fresh CashFlux install names
// its first workspace with the literal string "default"
// (internal/app/workspace.go). While workspaces.id was a GLOBAL primary key,
// that meant the first account ever to sync claimed the id for the whole server
// and every other account's first push was refused with `workspace not found` —
// terminal, because syncstate correctly treats that answer as a decision rather
// than something to retry. Two people on one server, and the second could never
// sync at all.
//
// Schema v17 made ownership part of the key. These tests hold that open.

func TestEveryAccountCanOwnTheDefaultWorkspace(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()

	ctxA := ContextWithAuthUser(context.Background(), AuthUser{ID: "user-A"})
	ctxB := ContextWithAuthUser(context.Background(), AuthUser{ID: "user-B"})

	if _, err := svc.PutWorkspace(ctxA, Workspace{ID: "default", Name: "Default"}, now, false, now); err != nil {
		t.Fatalf("first account: %v", err)
	}
	if _, err := svc.PutWorkspace(ctxB, Workspace{ID: "default", Name: "Default"}, now, false, now); err != nil {
		t.Fatalf("second account was refused its own default workspace: %v", err)
	}

	for _, userID := range []string{"user-A", "user-B"} {
		ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})
		list, err := svc.List(ctx, false)
		if err != nil {
			t.Fatalf("List(%s): %v", userID, err)
		}
		if len(list) != 1 || list[0].ID != "default" {
			t.Fatalf("%s owns %+v, want exactly its own default workspace", userID, list)
		}
	}
}

// Same id, separate data. The collision fix would be worthless — worse than
// worthless — if two accounts ended up sharing one snapshot row.
func TestSameWorkspaceIDKeepsSeparateSnapshots(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()

	for _, tc := range []struct{ user, dataset string }{
		{"user-A", `{"owner":"A"}`},
		{"user-B", `{"owner":"B"}`},
	} {
		ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: tc.user})
		if _, err := svc.PutWorkspace(ctx, Workspace{ID: "default", Name: "Default"}, now, false, now); err != nil {
			t.Fatalf("%s put workspace: %v", tc.user, err)
		}
		if err := store.PutSnapshot(Snapshot{
			UserID: tc.user, WorkspaceID: "default", Dataset: []byte(tc.dataset), Version: 1, UpdatedAt: now,
		}, 0, 5); err != nil {
			t.Fatalf("%s put snapshot: %v", tc.user, err)
		}
	}

	for _, tc := range []struct{ user, want string }{
		{"user-A", `{"owner":"A"}`},
		{"user-B", `{"owner":"B"}`},
	} {
		got, ok, err := store.GetSnapshotForUser(tc.user, "default")
		if err != nil || !ok {
			t.Fatalf("%s snapshot: ok=%v err=%v", tc.user, ok, err)
		}
		if string(got.Dataset) != tc.want {
			t.Fatalf("%s read %s, want %s — the two accounts share a snapshot row", tc.user, got.Dataset, tc.want)
		}
	}
}

// Deleting one account must not take the other's identically-named workspace
// with it. The child tables key on the owner too, so a delete scoped by
// workspace id alone would have reached across.
func TestDeletingOneAccountLeavesTheOthersDefaultWorkspace(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()

	for _, userID := range []string{"user-A", "user-B"} {
		ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})
		if _, err := svc.PutWorkspace(ctx, Workspace{ID: "default", Name: "Default"}, now, false, now); err != nil {
			t.Fatalf("%s: %v", userID, err)
		}
		if err := store.PutSnapshot(Snapshot{
			UserID: userID, WorkspaceID: "default", Dataset: []byte(`{"x":1}`), Version: 1, UpdatedAt: now,
		}, 0, 5); err != nil {
			t.Fatalf("%s snapshot: %v", userID, err)
		}
	}

	if _, err := store.DeleteAccount("user-A"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	if _, ok, _ := store.GetSnapshotForUser("user-A", "default"); ok {
		t.Fatal("the deleted account's snapshot survived")
	}
	if _, ok, err := store.GetSnapshotForUser("user-B", "default"); err != nil || !ok {
		t.Fatalf("deleting one account destroyed the other's snapshot: ok=%v err=%v", ok, err)
	}
	list, err := store.ListWorkspaces("user-B", true)
	if err != nil || len(list) != 1 {
		t.Fatalf("surviving account owns %d workspaces (err %v), want 1", len(list), err)
	}
}

// Attachment links are per-account too: two households can link the same
// content-addressed blob from a workspace they both call "default", and neither
// may read through the other's link.
func TestSameWorkspaceIDKeepsSeparateBlobLinks(t *testing.T) {
	store := openTestStore(t)
	svc := NewSyncService(store)
	now := time.Now().UTC()
	root := t.TempDir()

	blob, err := store.PutBlob(root, []byte("receipt"), "text/plain", "r.txt", 1<<20)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	for _, userID := range []string{"user-A", "user-B"} {
		ctx := ContextWithAuthUser(context.Background(), AuthUser{ID: userID})
		if _, err := svc.PutWorkspace(ctx, Workspace{ID: "default", Name: "Default"}, now, false, now); err != nil {
			t.Fatalf("%s: %v", userID, err)
		}
	}
	if err := store.LinkWorkspaceBlob("user-A", "default", blob.Hash); err != nil {
		t.Fatalf("LinkWorkspaceBlob: %v", err)
	}

	linked, err := store.UserWorkspaceBlob("user-A", "default", blob.Hash)
	if err != nil || !linked {
		t.Fatalf("owner cannot read its own link: linked=%v err=%v", linked, err)
	}
	linked, err = store.UserWorkspaceBlob("user-B", "default", blob.Hash)
	if err != nil {
		t.Fatalf("UserWorkspaceBlob(user-B): %v", err)
	}
	if linked {
		t.Fatal("the other account can read an attachment through its own identically-named workspace")
	}
}
