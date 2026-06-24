// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunRetentionPrunesAuditSnapshotHistoryAndBackups(t *testing.T) {
	dataDir := t.TempDir()
	store, err := OpenStore(filepath.Join(dataDir, "cashflux-server.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Date(2026, time.June, 19, 3, 10, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -40)
	recent := now.AddDate(0, 0, -2)

	if _, err := store.AppendAuditEvent(AuditEvent{Timestamp: old, ActorID: "u1", Action: "auth.login", TargetType: "user", TargetID: "u1"}); err != nil {
		t.Fatalf("append old audit: %v", err)
	}
	if _, err := store.AppendAuditEvent(AuditEvent{Timestamp: recent, ActorID: "u1", Action: "workspace.put", TargetType: "workspace", TargetID: "w1"}); err != nil {
		t.Fatalf("append recent audit: %v", err)
	}
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: old}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: old}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	if err := store.PutSnapshot(Snapshot{WorkspaceID: "w1", Dataset: []byte("old"), Version: 1, UpdatedAt: old}, 1024, 5); err != nil {
		t.Fatalf("PutSnapshot old: %v", err)
	}
	if err := store.PutSnapshot(Snapshot{WorkspaceID: "w1", Dataset: []byte("recent"), Version: 2, UpdatedAt: recent}, 1024, 5); err != nil {
		t.Fatalf("PutSnapshot recent: %v", err)
	}
	backupRoot := filepath.Join(dataDir, "backups")
	for _, name := range []string{
		"cashflux-backup-" + old.Format("20060102T150405Z"),
		"cashflux-backup-" + recent.Format("20060102T150405Z"),
		"notes",
	} {
		if err := os.MkdirAll(filepath.Join(backupRoot, name), 0o700); err != nil {
			t.Fatalf("mkdir backup %s: %v", name, err)
		}
	}

	result, err := RunRetention(context.Background(), store, RetentionOptions{
		DataDir:                      dataDir,
		AuditRetentionDays:           30,
		SnapshotHistoryRetentionDays: 30,
		BackupRetentionDays:          30,
		Now:                          now,
	})
	if err != nil {
		t.Fatalf("RunRetention: %v", err)
	}
	if result.AuditEventsDeleted != 1 || result.SnapshotHistoryDeleted != 1 || result.BackupDirectoriesDeleted != 1 {
		t.Fatalf("retention result = %+v", result)
	}
	events, err := store.ListAuditEvents(0, 10)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(events) != 1 || events[0].Action != "workspace.put" {
		t.Fatalf("remaining audit events = %+v", events)
	}
	history, err := store.SnapshotHistory("w1", 10)
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("snapshot history after prune = %+v", history)
	}
	if _, err := os.Stat(filepath.Join(backupRoot, "cashflux-backup-"+old.Format("20060102T150405Z"))); !os.IsNotExist(err) {
		t.Fatalf("old backup still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupRoot, "cashflux-backup-"+recent.Format("20060102T150405Z"))); err != nil {
		t.Fatalf("recent backup stat: %v", err)
	}
}

func TestRunRetentionHonorsCanceledContext(t *testing.T) {
	store := openTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunRetention(ctx, store, RetentionOptions{AuditRetentionDays: 1, Now: time.Now().UTC()}); err == nil {
		t.Fatal("RunRetention accepted canceled context")
	}
}
