// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOrphanBlobSweepRespectsMetadataAgeAndLayout(t *testing.T) {
	s := openTestStore(t)
	root := filepath.Join(t.TempDir(), "blobs")
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	before := now.Add(-OrphanBlobGracePeriod)
	makeBlob := func(data string, keepRow bool, modified time.Time) (string, string) {
		t.Helper()
		blob, err := s.PutBlob(root, []byte(data), "application/octet-stream", "", 1024)
		if err != nil {
			t.Fatal(err)
		}
		path, err := blobPath(root, blob.Hash)
		if err != nil {
			t.Fatal(err)
		}
		if !keepRow {
			if _, err := s.db.Exec(`DELETE FROM blobs WHERE hash = ?`, blob.Hash); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
		return blob.Hash, path
	}
	_, oldOrphan := makeBlob("old orphan", false, old)
	_, recentOrphan := makeBlob("recent orphan", false, now)
	_, linkedFile := makeBlob("tracked blob", true, old)
	invalid := filepath.Join(root, "aa", "bb", "not-a-blob")
	if err := os.MkdirAll(filepath.Dir(invalid), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.SweepOrphanBlobFiles(root, before); err != nil || deleted != 1 {
		t.Fatalf("first orphan sweep: deleted=%d err=%v", deleted, err)
	}
	if _, err := os.Stat(oldOrphan); !os.IsNotExist(err) {
		t.Fatalf("old orphan still present: %v", err)
	}
	for _, path := range []string{recentOrphan, linkedFile, invalid} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("sweep removed protected file %s: %v", path, err)
		}
	}
	if err := os.Chtimes(recentOrphan, old, old); err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.SweepOrphanBlobFiles(root, before); err != nil || deleted != 1 {
		t.Fatalf("aged orphan sweep: deleted=%d err=%v", deleted, err)
	}
}

func TestOrphanBlobSweepCannotRemoveConcurrentLinkedUpload(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "server.db")
	s, err := OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	gc, err := OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gc.Close() })
	root := filepath.Join(t.TempDir(), "blobs")
	now := time.Now().UTC()
	if err := s.UpsertUser(User{ID: "u1", Provider: "github", Subject: "alice", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutWorkspace(Workspace{ID: "w1", UserID: "u1", Name: "Home", UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-48 * time.Hour)
	for i := 0; i < 20; i++ {
		data := []byte{byte(i), 0x12, 0x93, 0x41}
		blob, err := s.PutBlob(root, data, "application/octet-stream", "", 1024)
		if err != nil {
			t.Fatal(err)
		}
		path, err := blobPath(root, blob.Hash)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(`DELETE FROM blobs WHERE hash = ?`, blob.Hash); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() {
			_, err := s.PutLinkedBlobContext(context.Background(), root, data, "application/octet-stream", "", 1024, "u1", "w1")
			result <- err
		}()
		if _, err := gc.SweepOrphanBlobFiles(root, now.Add(-OrphanBlobGracePeriod)); err != nil {
			t.Fatal(err)
		}
		if err := <-result; err != nil {
			t.Fatal(err)
		}
		stored, err := s.ReadBlob(root, blob.Hash)
		if err != nil || !bytes.Equal(stored, data) {
			t.Fatalf("linked blob lost during orphan GC: iteration=%d err=%v", i, err)
		}
	}
}
