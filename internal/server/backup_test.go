// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunBackupCopiesDatabaseBlobsAndManifest(t *testing.T) {
	dataDir := t.TempDir()
	store, err := OpenStore(filepath.Join(dataDir, "cashflux-server.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	now := time.Date(2026, time.June, 19, 3, 0, 0, 0, time.UTC)
	if err := store.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := store.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatalf("PutWorkspace: %v", err)
	}
	blob, err := store.PutBlob(filepath.Join(dataDir, "blobs"), []byte("receipt"), "text/plain", "receipt.txt", 1024)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if err := store.LinkWorkspaceBlob("w1", blob.Hash); err != nil {
		t.Fatalf("LinkWorkspaceBlob: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	backupDir, manifest, err := RunBackup(context.Background(), BackupOptions{
		DataDir: dataDir,
		OutDir:  filepath.Join(t.TempDir(), "backups"),
		Now:     now,
	})
	if err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	if filepath.Base(backupDir) != "cashflux-backup-20260619T030000Z" {
		t.Fatalf("backup dir = %s", backupDir)
	}
	if manifest.ServerSchemaVersion != CurrentServerSchemaVersion || len(manifest.Files) != 2 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if manifest.RPO == "" || manifest.RTO == "" {
		t.Fatalf("manifest missing recovery objectives: %+v", manifest)
	}

	manifestData, err := os.ReadFile(filepath.Join(backupDir, backupManifestName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var fromDisk BackupManifest
	if err := json.Unmarshal(manifestData, &fromDisk); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(fromDisk.Files) != len(manifest.Files) {
		t.Fatalf("manifest file count = %d, want %d", len(fromDisk.Files), len(manifest.Files))
	}

	restored, err := OpenStore(filepath.Join(backupDir, "cashflux-server.db"))
	if err != nil {
		t.Fatalf("open restored store: %v", err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	if _, ok, err := restored.GetWorkspace("u1", "w1"); err != nil || !ok {
		t.Fatalf("restored workspace = ok %v err %v", ok, err)
	}
	got, err := restored.ReadBlob(filepath.Join(backupDir, "blobs"), blob.Hash)
	if err != nil {
		t.Fatalf("read restored blob: %v", err)
	}
	if string(got) != "receipt" {
		t.Fatalf("restored blob = %q", got)
	}
	blobRel, err := filepath.Rel(dataDir, mustBlobPath(t, filepath.Join(dataDir, "blobs"), blob.Hash))
	if err != nil {
		t.Fatalf("blob rel: %v", err)
	}
	if manifestFile(manifest, filepath.ToSlash(blobRel)).SHA256 == "" {
		t.Fatalf("manifest missing blob digest: %+v", manifest)
	}
}

func TestRunBackupHonorsCanceledContext(t *testing.T) {
	dataDir := t.TempDir()
	store, err := OpenStore(filepath.Join(dataDir, "cashflux-server.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := RunBackup(ctx, BackupOptions{DataDir: dataDir, OutDir: t.TempDir()}); err == nil {
		t.Fatal("RunBackup accepted canceled context")
	}
}

func manifestFile(manifest BackupManifest, path string) BackupFileInfo {
	for _, file := range manifest.Files {
		if file.Path == path {
			return file
		}
	}
	return BackupFileInfo{}
}

func mustBlobPath(t *testing.T, root, hash string) string {
	t.Helper()
	path, err := blobPath(root, hash)
	if err != nil {
		t.Fatalf("blobPath: %v", err)
	}
	return path
}
