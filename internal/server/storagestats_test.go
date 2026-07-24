// SPDX-License-Identifier: MIT

package server

import "testing"

// TestStorageStats proves the reported db/blob sizes are real, non-zero
// numbers that reflect actual stored data — the admin console's storage
// panel (Cam: "I wanna see... the database and artifact blob storage
// size").
func TestStorageStats(t *testing.T) {
	store := openTestStore(t)
	root := t.TempDir()

	dbBefore, blobBefore, err := store.StorageStats()
	if err != nil {
		t.Fatalf("StorageStats: %v", err)
	}
	if dbBefore <= 0 {
		t.Fatalf("StorageStats: db size = %d, want > 0 for a migrated database", dbBefore)
	}
	if blobBefore != 0 {
		t.Fatalf("StorageStats: blob size = %d, want 0 before any blob is stored", blobBefore)
	}

	payload := []byte("some artifact content for the storage stats test")
	if _, err := store.PutBlob(root, payload, "text/plain", "note.txt", 1<<20); err != nil {
		t.Fatalf("PutBlob: %v", err)
	}

	_, blobAfter, err := store.StorageStats()
	if err != nil {
		t.Fatalf("StorageStats after PutBlob: %v", err)
	}
	if blobAfter != int64(len(payload)) {
		t.Fatalf("StorageStats: blob size = %d, want %d (the one stored blob's byte length)", blobAfter, len(payload))
	}
}
