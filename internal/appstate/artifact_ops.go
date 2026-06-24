// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/artifactstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

// SetBlobStore wires in an IndexedDB-backed binary blob store so that artifact
// image bytes are kept out of the main localStorage JSON blob. Called once by
// the wasm entry point after successfully opening IndexedDB. A nil argument
// clears the blob store (falls back to embedding bytes in the dataset).
func (a *App) SetBlobStore(s artifactstore.Store) { a.blobs = s }

// BlobStoreUsage returns the last cached blob-store usage in bytes. The value
// is updated by RefreshBlobUsage, which must be called from outside the render
// path (e.g. after PutArtifact, DeleteArtifact, or on a background ticker).
// Returns 0 when no blob store is wired in or usage has not yet been queried.
// Safe to call from render functions — never blocks.
func (a *App) BlobStoreUsage() int64 {
	return a.blobUsageCache
}

// RefreshBlobUsage queries the blob store for its current usage and updates the
// cached value returned by BlobStoreUsage. This must be called from outside the
// render path (the IDB Usage call is async and would deadlock in the wasm render
// goroutine). Callers: initBlobStore (initial probe), PutArtifact, DeleteArtifact.
func (a *App) RefreshBlobUsage() {
	if a.blobs == nil {
		return
	}
	n, err := a.blobs.Usage()
	if err != nil {
		a.log.Warn("blob store usage query failed", "err", err)
		return
	}
	a.blobUsageCache = n
}

// StoreBlobForArtifact moves the binary bytes for an image artifact into the
// blob store, stripping them from the artifact record so the main dataset JSON
// stays small. If the blob store is unavailable, the bytes remain on the
// artifact (the inline fallback path). Returns the modified artifact (bytes
// cleared if the put succeeded) so the caller can decide what to persist.
func (a *App) StoreBlobForArtifact(art domain.Artifact) (domain.Artifact, error) {
	if a.blobs == nil || art.Kind != "image" || len(art.Bytes) == 0 {
		return art, nil
	}
	if err := a.blobs.Put(art.ID, art.MIME, art.Bytes); err != nil {
		return art, fmt.Errorf("appstate: store blob for artifact %s: %w", art.ID, err)
	}
	art.Bytes = nil
	return art, nil
}

// GetBlobForArtifact fetches binary bytes from the blob store for the given
// artifact ID. Returns (nil, nil) when the blob store is not wired in or the
// ID is not found. An error is returned only for actual storage failures.
func (a *App) GetBlobForArtifact(id string) ([]byte, error) {
	if a.blobs == nil {
		return nil, nil
	}
	_, data, ok, err := a.blobs.Get(id)
	if err != nil {
		return nil, fmt.Errorf("appstate: get blob for artifact %s: %w", id, err)
	}
	if !ok {
		return nil, nil
	}
	return data, nil
}

// rehydrateArtifactBytes fills in Bytes for each artifact that has none stored
// in the dataset by fetching them from the blob store. It is a no-op when blobs
// is nil, when the artifact already carries bytes, or when the artifact kind is
// not "image". This is safe to call only from non-render paths (export/import)
// because the wasm IDB implementation blocks on a channel — calling it from a
// render function would deadlock the single-threaded wasm runtime.
func (a *App) rehydrateArtifactBytes(arts []domain.Artifact) {
	if a.blobs == nil {
		return
	}
	for i := range arts {
		if len(arts[i].Bytes) > 0 {
			continue // already present (legacy record or CSV artifact)
		}
		if arts[i].Kind != "image" {
			continue // only binary artifacts need rehydration
		}
		_, data, ok, err := a.blobs.Get(arts[i].ID)
		if err != nil {
			a.log.Warn("blob store get for export failed", "id", arts[i].ID, "err", err)
			continue
		}
		if ok {
			arts[i].Bytes = data
		}
	}
}

// ExportJSONWithBlobs serializes the whole dataset, gathering artifact image
// bytes from the blob store so the backup is fully self-contained (an import
// on a fresh device works without access to the original IndexedDB). This is
// the export path for both manual backups and the autosave (redacted) snapshot.
// Call from a goroutine or event handler, never from a render function.
func (a *App) ExportJSONWithBlobs() ([]byte, error) {
	ds, err := a.store.Snapshot()
	if err != nil {
		return nil, err
	}
	a.rehydrateArtifactBytes(ds.Artifacts)
	return store.Export(ds)
}

// ExportJSONRedactedWithBlobs is like ExportJSONWithBlobs but strips the
// OpenAI key so the autosaved dataset snapshot never writes the secret to
// localStorage.
func (a *App) ExportJSONRedactedWithBlobs() ([]byte, error) {
	ds, err := a.store.Snapshot()
	if err != nil {
		return nil, err
	}
	ds.Settings.OpenAIKey = ""
	a.rehydrateArtifactBytes(ds.Artifacts)
	return store.Export(ds)
}

// ImportJSONWithBlobs replaces all data with the given dataset JSON. When a
// blob store is wired in, artifact image bytes embedded in the import are moved
// to IndexedDB and cleared from the in-memory record, so the next autosave does
// not write them back into localStorage.
func (a *App) ImportJSONWithBlobs(data []byte) error {
	ds, err := store.Import(data)
	if err != nil {
		return err
	}
	// Move image bytes to the blob store before loading, so SQLite stores only
	// the lightweight record.
	if a.blobs != nil {
		for i := range ds.Artifacts {
			art := &ds.Artifacts[i]
			if art.Kind == "image" && len(art.Bytes) > 0 {
				if putErr := a.blobs.Put(art.ID, art.MIME, art.Bytes); putErr != nil {
					a.log.Warn("blob store put during import failed", "id", art.ID, "err", putErr)
				} else {
					art.Bytes = nil // strip from the SQLite record
				}
			}
		}
	}
	if err := a.store.Load(ds); err != nil {
		return err
	}
	a.log.Info("imported dataset with blob migration", "accounts", len(ds.Accounts), "transactions", len(ds.Transactions), "artifacts", len(ds.Artifacts))
	return nil
}

// DatasetBytesWithBlobs reports the combined storage footprint: the serialized
// dataset JSON (what goes to localStorage) plus the blob store usage (IndexedDB
// artifact bytes). The UI uses this for a storage meter. Returns 0 on errors
// (logged). When a blob store is wired in, the JSON will not contain artifact
// bytes (they moved to IndexedDB), so both values are summed.
func (a *App) DatasetBytesWithBlobs() int {
	ds, err := a.store.Snapshot()
	if err != nil {
		a.logErr("datasetBytesWithBlobs", err)
		return 0
	}
	ds.Settings.OpenAIKey = ""
	// Do NOT rehydrate artifact bytes here — we want the size of what actually
	// lands in localStorage (blob bytes live in IndexedDB, not localStorage).
	b, err := store.Export(ds)
	if err != nil {
		a.logErr("datasetBytesWithBlobs", err)
		return 0
	}
	total := len(b)
	total += int(a.BlobStoreUsage())
	return total
}
