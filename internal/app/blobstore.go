// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/artifactstore"
	"github.com/monstercameron/CashFlux/internal/browserstore"
)

var localEncryptedArtifactStore artifactstore.EncryptedStore

// initBlobStore opens the IndexedDB artifact store and wires it into the app.
// If IndexedDB is unavailable or the open fails, the app continues without it
// (artifact bytes fall back to being stored inline in the SQLite/localStorage
// dataset), so this is always safe to call.
func initBlobStore() {
	app := appstate.Default
	if app == nil {
		return
	}
	idb, err := artifactstore.OpenIDB()
	if err != nil {
		app.Log().Warn("IndexedDB artifact store unavailable; falling back to inline bytes", "err", err)
		return
	}
	localEncryptedArtifactStore = artifactstore.EncryptedStore{Raw: idb, Cipher: browserstore.LocalArtifactCipher{}}
	app.SetBlobStore(localEncryptedArtifactStore)
	app.Log().Info("encrypted IndexedDB artifact store ready")
	// Prime the usage cache so the first render of the Artifacts screen shows a
	// non-zero figure. This runs in the boot goroutine (not the render path) so
	// the IDB async call can complete normally.
	app.RefreshBlobUsage()
}

// Migrate files referenced by every saved workspace after protected hydration.
// The encrypted wrapper also migrates on ordinary reads, so a sync or download
// remains safe if it races this background pass. The original record is never
// deleted before its encrypted replacement commits.
func migrateReferencedArtifactBlobs() {
	app := appstate.Default
	if app == nil {
		return
	}
	activeSaved := hadLocalDataset
	go func() {
		if localEncryptedArtifactStore.Raw != nil {
			migrated, err := localEncryptedArtifactStore.MigrateAllLegacy()
			if err != nil {
				app.Log().Warn("origin-wide local artifact migration paused", "migrated", migrated, "err", err)
			} else if migrated > 0 {
				app.Log().Info("encrypted legacy local artifact records", "count", migrated)
			}
		}
		if activeSaved {
			for _, art := range app.Artifacts() {
				if !artifacts.IsBinaryKind(art.Kind) || len(art.Bytes) > 0 || (art.Size == 0 && art.BlobRef == nil) {
					continue
				}
				if data, err := app.GetBlobForArtifact(art.ID); err != nil {
					app.Log().Warn("local artifact migration paused", "id", art.ID, "err", err)
				} else if len(data) == 0 {
					app.Log().Warn("local artifact migration found missing bytes", "id", art.ID)
				}
			}
		}
		r := loadRegistry()
		for _, w := range r.Workspaces {
			if w.ID == r.ActiveID {
				continue
			}
			data := loadBlob(w.ID)[datasetStoreKey]
			if data == "" {
				continue
			}
			if _, err := app.CompleteArtifactSnapshot([]byte(data)); err != nil {
				app.Log().Warn("inactive workspace artifact migration paused", "workspace", w.ID, "err", err)
			}
		}
	}()
}
