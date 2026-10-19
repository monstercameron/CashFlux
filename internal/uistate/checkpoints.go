// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"errors"
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

var checkpointCommit = browserstore.SetSecureBatch

// SetCheckpointCommitForTest replaces the checkpoint storage callback so WASM
// integration tests can hold or fail a commit. The returned function restores
// the previous callback and should be deferred immediately by the test.
func SetCheckpointCommitForTest(commit func(values, expected map[string]string, done func(error))) func() {
	previous := checkpointCommit
	if commit == nil {
		checkpointCommit = browserstore.SetSecureBatch
	} else {
		checkpointCommit = commit
	}
	return func() { checkpointCommit = previous }
}

// Checkpoints returns the saved checkpoint ring, oldest-first.
func Checkpoints() []checkpoint.Checkpoint {
	return checkpoint.DecodeIndex(browserstore.GetString(ckptIndexKey))
}

// SaveCheckpoint starts a snapshot under the given plain-English label and
// returns its pending ID ("" on export failure). Returning an ID does not mean
// the write committed. Oldest checkpoints beyond the ring cap are evicted.
//
// Deprecated: use SaveCheckpointThen and wait for durable success before a risky
// operation, including imports that associate a rollback checkpoint with changes.
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

// SaveCheckpointThen commits the snapshot and index atomically before returning
// its ID through done. Callers must begin the protected operation only when err
// is nil. Unlike SaveCheckpoint's pending handle, this callback is a save receipt.
func SaveCheckpointThen(label string, done func(checkpointID string, err error)) {
	finish := func(id string, err error) {
		if done != nil {
			done(id, err)
		}
	}
	app := appstate.Default
	if app == nil {
		finish("", errors.New("checkpoint: no household is open"))
		return
	}
	data, err := app.ExportJSONRedacted()
	if err != nil {
		finish("", err)
		return
	}
	previous := browserstore.GetString(ckptIndexKey)
	cp := checkpoint.Checkpoint{ID: id.New(), At: time.Now(), Label: label, Size: len(data)}
	kept, dropped := checkpoint.Push(checkpoint.DecodeIndex(previous), cp, checkpoint.MaxEntries)
	checkpointCommit(map[string]string{
		ckptBlobPrefix + cp.ID: string(data), ckptIndexKey: checkpoint.EncodeIndex(kept),
	}, map[string]string{ckptIndexKey: previous}, func(err error) {
		if err != nil {
			app.Log().Error("checkpoint commit failed", "err", err)
			finish("", err)
			return
		}
		// Eviction happens after the new snapshot is durable. A failed cleanup
		// may retain an old encrypted blob, but cannot destroy the new checkpoint.
		for _, old := range dropped {
			browserstore.Remove(ckptBlobPrefix + old.ID)
		}
		finish(cp.ID, nil)
	})
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
