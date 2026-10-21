// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/artifactstore"
	"github.com/monstercameron/CashFlux/internal/browserstore"
)

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
	app.SetBlobStore(artifactstore.EncryptedStore{Raw: idb, Cipher: browserstore.LocalArtifactCipher{}})
	app.Log().Info("encrypted IndexedDB artifact store ready")
	// Prime the usage cache so the first render of the Artifacts screen shows a
	// non-zero figure. This runs in the boot goroutine (not the render path) so
	// the IDB async call can complete normally.
	app.RefreshBlobUsage()
}

// Migrate files referenced by the active household after protected hydration.
// The encrypted wrapper also migrates on ordinary reads, so a sync or download
// remains safe if it races this background pass. Inactive workspace bundles are
// handled when opened; the original record is never deleted before replacement.
func migrateActiveArtifactBlobs() {
	app := appstate.Default
	if app == nil || !hadLocalDataset {
		return
	}
	go func() {
		for _, art := range app.Artifacts() {
			if !artifacts.IsBinaryKind(art.Kind) || len(art.Bytes) > 0 || art.Size == 0 {
				continue
			}
			if _, err := app.GetBlobForArtifact(art.ID); err != nil {
				app.Log().Warn("local artifact migration paused", "id", art.ID, "err", err)
			}
		}
	}()
}
