// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/checkpoint"
	"github.com/monstercameron/CashFlux/internal/id"
)

// Pre-operation safety checkpoints (#55): before a risky bulk operation the
// caller snapshots the whole dataset (the redacted export — the same blob the
// autosave writes, so the AI key never lands in browser storage) into a small
// capped ring in the browser store. Blobs live OUTSIDE the SQLite dataset for
// the same reason the workspace registry does (kvbridge.go): a dataset can't
// usefully contain rollback copies of itself — each export would swallow the
// previous checkpoints and balloon.

const (
	ckptIndexKey   = "cashflux:checkpoints"
	ckptBlobPrefix = "cashflux:checkpoint-data:"
)

// Checkpoints returns the saved checkpoint ring, oldest-first.
func Checkpoints() []checkpoint.Checkpoint {
	return checkpoint.DecodeIndex(browserstore.GetString(ckptIndexKey))
}

// SaveCheckpoint snapshots the current dataset under the given plain-English
// label ("Before applying rules to 12 transactions") and returns the new
// checkpoint's ID ("" on failure) so callers can associate the snapshot with
// what they're about to do (#57: per-import roll-back). Oldest checkpoints
// beyond the ring cap are deleted, blob and all.
func SaveCheckpoint(label string) string {
	app := appstate.Default
	if app == nil {
		return ""
	}
	data, err := app.ExportJSONRedacted()
	if err != nil {
		app.Log().Error("checkpoint: export failed", "err", err)
		return ""
	}
	cp := checkpoint.Checkpoint{ID: id.New(), At: time.Now(), Label: label, Size: len(data)}
	kept, dropped := checkpoint.Push(Checkpoints(), cp, checkpoint.MaxEntries)
	for _, d := range dropped {
		browserstore.Remove(ckptBlobPrefix + d.ID)
	}
	browserstore.Set(ckptBlobPrefix+cp.ID, string(data))
	browserstore.Set(ckptIndexKey, checkpoint.EncodeIndex(kept))
	return cp.ID
}

// HasCheckpoint reports whether cpID is still in the ring (its blob restorable).
func HasCheckpoint(cpID string) bool {
	if cpID == "" {
		return false
	}
	_, ok := checkpoint.Find(Checkpoints(), cpID)
	return ok
}

// RestoreCheckpoint replaces the live dataset with the snapshot saved under
// cpID and reports success. The checkpoint itself is kept (a restore you can
// re-restore), and the UI re-renders + persists.
func RestoreCheckpoint(cpID string) bool {
	app := appstate.Default
	if app == nil {
		return false
	}
	blob, ok := browserstore.Get(ckptBlobPrefix + cpID)
	if !ok || blob == "" {
		return false
	}
	if err := app.ImportJSON([]byte(blob)); err != nil {
		app.Log().Error("checkpoint: restore failed", "err", err)
		return false
	}
	BumpDataRevision()
	RequestPersist()
	return true
}

// DeleteCheckpoint removes one checkpoint (index entry + blob).
func DeleteCheckpoint(cpID string) {
	idx, found := checkpoint.Remove(Checkpoints(), cpID)
	if !found {
		return
	}
	browserstore.Remove(ckptBlobPrefix + cpID)
	browserstore.Set(ckptIndexKey, checkpoint.EncodeIndex(idx))
}
