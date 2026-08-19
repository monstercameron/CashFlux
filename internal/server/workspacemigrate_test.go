// SPDX-License-Identifier: MIT

package server

import (
	"testing"
	"time"
)

// These tests exist because this is the code that moves somebody's household
// records between accounts. The interesting cases are not the happy path — they
// are the refusals, and the question of whether anything is recoverable when an
// operator picks the wrong row.

func migrateTestUsers(t *testing.T, store *Store, ids ...string) {
	t.Helper()
	now := time.Now().UTC()
	for _, id := range ids {
		if err := store.UpsertUser(User{ID: id, Provider: "device", Subject: id, CreatedAt: now}); err != nil {
			t.Fatalf("UpsertUser(%s): %v", id, err)
		}
	}
}

func seedWorkspace(t *testing.T, store *Store, userID, wsID, name string, dataset []byte) {
	t.Helper()
	now := time.Now().UTC()
	if err := store.PutWorkspace(Workspace{ID: wsID, UserID: userID, Name: name, UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace(%s): %v", wsID, err)
	}
	if len(dataset) > 0 {
		if err := store.PutSnapshot(Snapshot{UserID: userID, WorkspaceID: wsID, Dataset: dataset, Version: 1, UpdatedAt: now}, 0, 10); err != nil {
			t.Fatalf("PutSnapshot(%s): %v", wsID, err)
		}
	}
}

func TestTransferWorkspaceMovesOwnershipWithoutCopying(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "user-old", "user-new")
	seedWorkspace(t, store, "user-old", "ws-1", "Household", []byte(`{"txns":1}`))
	// BOTH sides are locked: an unsuspended SOURCE can land a sync after the
	// transfer commits and flip ownership back (see ErrMigrationTargetUnlocked).
	suspendAll(t, store, "user-old", "user-new")

	res, err := store.TransferWorkspace("user-old", "user-new", "ws-1", time.Now().UTC(), true)
	if err != nil {
		t.Fatalf("TransferWorkspace: %v", err)
	}
	if res.TargetUserID != "user-new" {
		t.Fatalf("result = %+v", res)
	}
	// The new owner can read it...
	if _, found, err := store.GetWorkspace("user-new", "ws-1"); err != nil || !found {
		t.Fatalf("new owner cannot see the workspace: found=%v err=%v", found, err)
	}
	// ...the old one cannot...
	if _, found, err := store.GetWorkspace("user-old", "ws-1"); err != nil || found {
		t.Fatalf("old owner still sees the workspace: found=%v err=%v", found, err)
	}
	// ...and the DATA came with it, under the same workspace id. This is the
	// whole reason a transfer beats export/import: every other device pinned to
	// this id keeps working.
	snap, ok, err := store.GetSnapshotForUser("user-new", "ws-1")
	if err != nil || !ok {
		t.Fatalf("snapshot did not follow the workspace: ok=%v err=%v", ok, err)
	}
	if string(snap.Dataset) != `{"txns":1}` {
		t.Fatalf("dataset = %q", snap.Dataset)
	}
}

func TestTransferWorkspaceRefusesTheDangerousCases(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "user-old", "user-new")
	seedWorkspace(t, store, "user-old", "ws-1", "Household", []byte(`{}`))
	now := time.Now().UTC()

	// An unlocked target: a device writing to it mid-transfer would race the
	// ownership change.
	if _, err := store.TransferWorkspace("user-old", "user-new", "ws-1", now, true); err != ErrMigrationTargetUnlocked {
		t.Fatalf("unlocked target: err = %v, want ErrMigrationTargetUnlocked", err)
	}
	// An unlocked SOURCE is just as dangerous, and used to be allowed: the
	// source's own device could still be syncing, and SyncService.PutWorkspace
	// upserts ownership without an ownership guard, so its write would land
	// after the transfer and silently undo it.
	if err := store.SetUserSuspended("user-new", true, now); err != nil {
		t.Fatalf("SetUserSuspended: %v", err)
	}
	if _, err := store.TransferWorkspace("user-old", "user-new", "ws-1", now, true); err != ErrMigrationTargetUnlocked {
		t.Fatalf("unlocked source: err = %v, want ErrMigrationTargetUnlocked", err)
	}
	if err := store.SetUserSuspended("user-old", true, now); err != nil {
		t.Fatalf("SetUserSuspended: %v", err)
	}
	// A target that does not exist.
	if _, err := store.TransferWorkspace("user-old", "nobody", "ws-1", now, false); err != ErrMigrationTargetMissing {
		t.Fatalf("missing target: err = %v, want ErrMigrationTargetMissing", err)
	}
	// Same account both sides.
	if _, err := store.TransferWorkspace("user-old", "user-old", "ws-1", now, false); err != ErrMigrationSameAccount {
		t.Fatalf("same account: err = %v, want ErrMigrationSameAccount", err)
	}
	// A workspace the source does not own — the shape of a stale console view.
	if _, err := store.TransferWorkspace("user-new", "user-old", "ws-1", now, false); err != ErrMigrationSourceMissing {
		t.Fatalf("wrong source: err = %v, want ErrMigrationSourceMissing", err)
	}
}

func TestTransferIsConditionalOnCurrentOwnership(t *testing.T) {
	// Two operators resolving the same case must not both succeed: the second
	// transfer would move a workspace that had already moved.
	store := openTestStore(t)
	migrateTestUsers(t, store, "a", "b", "c")
	seedWorkspace(t, store, "a", "ws-1", "Household", []byte(`{}`))
	now := time.Now().UTC()

	if _, err := store.TransferWorkspace("a", "b", "ws-1", now, false); err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	if _, err := store.TransferWorkspace("a", "c", "ws-1", now, false); err != ErrMigrationSourceMissing {
		t.Fatalf("replayed transfer: err = %v, want ErrMigrationSourceMissing", err)
	}
	if _, found, _ := store.GetWorkspace("b", "ws-1"); !found {
		t.Fatal("the workspace should still belong to the first target")
	}
}

func TestReplaceArchivesWhatItOverwrites(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "New copy", []byte(`{"keep":"this"}`))
	seedWorkspace(t, store, "dst", "ws-dst", "Old copy", []byte(`{"was":"here"}`))
	now := time.Now().UTC()

	res, err := store.ReplaceWorkspaceSnapshot("src", "ws-src", "dst", "ws-dst", now, false)
	if err != nil {
		t.Fatalf("ReplaceWorkspaceSnapshot: %v", err)
	}
	snap, ok, err := store.GetSnapshotForUser("dst", "ws-dst")
	if err != nil || !ok {
		t.Fatalf("target snapshot: ok=%v err=%v", ok, err)
	}
	if string(snap.Dataset) != `{"keep":"this"}` {
		t.Fatalf("target dataset = %q, want the source copy", snap.Dataset)
	}
	// The version must move with the contents, or the next client pull compares
	// against a stale version, decides it is up to date, and never sees the
	// overwrite performed for it.
	if snap.Version <= 1 {
		t.Fatalf("version = %d, want it advanced past the replaced copy", snap.Version)
	}
	// And what was there is recoverable — the archive is written in the same
	// transaction as the overwrite, so this is a guarantee and not a courtesy.
	restored, err := store.RollbackWorkspaceSnapshot("dst", "ws-dst", res.ArchivedVersion, now)
	if err != nil {
		t.Fatalf("RollbackWorkspaceSnapshot: %v", err)
	}
	back, ok, err := store.GetSnapshotForUser("dst", "ws-dst")
	if err != nil || !ok {
		t.Fatalf("after rollback: ok=%v err=%v", ok, err)
	}
	if string(back.Dataset) != `{"was":"here"}` {
		t.Fatalf("rollback restored %q, want the original contents", back.Dataset)
	}
	if restored.WorkspaceID != "ws-dst" {
		t.Fatalf("rollback result = %+v", restored)
	}
	// The rollback is itself reversible: it archived what it replaced, so an
	// undo cannot become a second incident.
	if again, err := store.RollbackWorkspaceSnapshot("dst", "ws-dst", restored.ArchivedVersion, now); err != nil {
		t.Fatalf("re-rollback: %v", err)
	} else if again.WorkspaceID != "ws-dst" {
		t.Fatalf("re-rollback result = %+v", again)
	}
}

func TestReplaceRefusesToEraseWithNothing(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "Empty", nil)
	seedWorkspace(t, store, "dst", "ws-dst", "Real data", []byte(`{"records":"many"}`))
	now := time.Now().UTC()

	if _, err := store.ReplaceWorkspaceSnapshot("src", "ws-src", "dst", "ws-dst", now, false); err == nil {
		t.Fatal("replacing with an empty source must be refused, not performed")
	}
	snap, _, _ := store.GetSnapshotForUser("dst", "ws-dst")
	if string(snap.Dataset) != `{"records":"many"}` {
		t.Fatalf("the target was damaged by a refused replace: %q", snap.Dataset)
	}
}

func TestPreviewBlocksRatherThanGuessing(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-1", "Household", []byte(`{"a":1}`))

	// A preview of something impossible reports WHY, so the console can say it
	// rather than offering a button that will fail.
	p, err := store.PreviewMigration(MigrateTransfer, "src", "nobody", "ws-1")
	if err != nil {
		t.Fatalf("PreviewMigration: %v", err)
	}
	if !p.Blocked || p.Reason == "" {
		t.Fatalf("preview = %+v, want blocked with a reason", p)
	}

	p, err = store.PreviewMigration(MigrateTransfer, "src", "dst", "ws-1")
	if err != nil {
		t.Fatalf("PreviewMigration: %v", err)
	}
	if p.Blocked {
		t.Fatalf("a legitimate transfer was blocked: %+v", p)
	}
	if p.SourceBytes != len(`{"a":1}`) || p.WorkspaceName != "Household" {
		t.Fatalf("preview = %+v, want the real counts an operator decides on", p)
	}
}

func TestPreviewReplaceWarnsAboutALossyOverwrite(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "Small", []byte(`{"a":1}`))
	seedWorkspace(t, store, "dst", "ws-dst", "Large", []byte(`{"a":1,"b":2,"c":3,"d":4}`))

	p, err := store.PreviewReplace("src", "ws-src", "dst", "ws-dst")
	if err != nil {
		t.Fatalf("PreviewReplace: %v", err)
	}
	// Warned, not blocked: replacing with a smaller dataset is often exactly
	// the repair being attempted, and refusing it would make a real situation
	// unfixable. The operator has to SEE it first.
	if p.Blocked {
		t.Fatalf("a smaller replacement was blocked outright: %+v", p)
	}
	if len(p.Warnings) == 0 {
		t.Fatal("overwriting a larger dataset must warn before it is confirmed")
	}
}

// suspendAll locks every named account, the precondition a migration requires.
func suspendAll(t *testing.T, store *Store, ids ...string) {
	t.Helper()
	now := time.Now().UTC()
	for _, id := range ids {
		if err := store.SetUserSuspended(id, true, now); err != nil {
			t.Fatalf("SetUserSuspended(%s): %v", id, err)
		}
	}
}

func TestReplaceCarriesAttachmentLinksToTheTarget(t *testing.T) {
	// The dataset carries blob HASHES, but permission to fetch the bytes comes
	// from a workspace_blobs row for the TARGET workspace. Copying the dataset
	// without the links leaves every attachment referenced-but-unreadable for
	// the new owner - and once the source is cleaned up, the sweeper deletes the
	// bytes outright. Found by adversarial review, 2026-08-17.
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "With attachments", []byte(`{"artifact":"h1"}`))
	seedWorkspace(t, store, "dst", "ws-dst", "Target", []byte(`{"old":true}`))
	now := time.Now().UTC()
	blob, err := store.PutBlob(t.TempDir(), []byte("receipt"), "image/png", "receipt.png", 1024)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if err := store.LinkWorkspaceBlob("src", "ws-src", blob.Hash); err != nil {
		t.Fatalf("LinkWorkspaceBlob: %v", err)
	}

	if _, err := store.ReplaceWorkspaceSnapshot("src", "ws-src", "dst", "ws-dst", now, false); err != nil {
		t.Fatalf("ReplaceWorkspaceSnapshot: %v", err)
	}
	if ok, err := store.UserWorkspaceBlob("dst", "ws-dst", blob.Hash); err != nil || !ok {
		t.Fatalf("target cannot reach the attachment its dataset references: ok=%v err=%v", ok, err)
	}
	// The source keeps its own link: the links are copied, not moved, so the
	// account the data came from is not broken by the repair either.
	if ok, err := store.UserWorkspaceBlob("src", "ws-src", blob.Hash); err != nil || !ok {
		t.Fatalf("source lost its attachment link: ok=%v err=%v", ok, err)
	}
}

func TestUnknownMigrationModeIsRefusedNotGuessed(t *testing.T) {
	// An unrecognised mode used to fall through to a TRANSFER, so a typo ran a
	// different operation than the one requested - and the audit line was built
	// from the raw string, so the record could name something that never
	// happened.
	for _, raw := range []string{"Replace", "REPLACE", "move", "delete"} {
		if _, ok := parseMigrationMode(raw); ok {
			t.Errorf("parseMigrationMode(%q) accepted an unknown mode", raw)
		}
	}
	for _, tc := range []struct {
		raw  string
		want MigrationMode
	}{
		{"", MigrateTransfer},
		{"transfer", MigrateTransfer},
		{" transfer ", MigrateTransfer},
		{"replace", MigrateReplace},
	} {
		got, ok := parseMigrationMode(tc.raw)
		if !ok || got != tc.want {
			t.Errorf("parseMigrationMode(%q) = (%q, %v), want (%q, true)", tc.raw, got, ok, tc.want)
		}
	}
}
