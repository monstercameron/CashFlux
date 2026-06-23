//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifactstore"
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
	app.SetBlobStore(idb)
	app.Log().Info("IndexedDB artifact store ready")
	// Prime the usage cache so the first render of the Artifacts screen shows a
	// non-zero figure. This runs in the boot goroutine (not the render path) so
	// the IDB async call can complete normally.
	app.RefreshBlobUsage()
}
