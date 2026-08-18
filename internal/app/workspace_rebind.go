// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"errors"
	"strings"

	"github.com/monstercameron/CashFlux/internal/workspace"
)

// Repointing a local workspace at a different remote id (TODOS.md C694).
//
// A workspace's id is not a label — it is the address the server knows it by.
// When a device ends up signed in as an account that does not own that address,
// the data is fine and the address is wrong, and the repair is to change the
// address while leaving the data exactly where it is. That is deliberately not
// "export and re-import": an import creates a NEW workspace, which loses the
// registry position, the per-workspace UI state bundled under the old key, and
// any relationship to what is already on the server.

// errRebindIncomplete is returned when a rebind is asked for without both ends.
var errRebindIncomplete = errors.New("rebind: both the current and the target workspace are required")

// errRebindTargetInUse is returned when the target id already names a local
// workspace. Merging two local workspaces is a different operation with
// different consequences, and doing it silently here would fold one household's
// data into another's under the cover of a connection repair.
var errRebindTargetInUse = errors.New("rebind: this device already has a workspace with that id")

// activeWorkspaceID is the id of the workspace currently loaded.
func activeWorkspaceID() string {
	r := loadRegistry()
	if w, ok := r.Active(); ok {
		return w.ID
	}
	return r.ActiveID
}

// renameWorkspaceID changes the id a local workspace is known by, keeping its
// name, colour, position, data and per-workspace UI state.
//
// The registry has no "change id" operation — ids are permanent by design — so
// this rebuilds the entry in place, preserving order. Everything else about the
// workspace is carried across by moving the one blob key its bundled state
// lives under.
func renameWorkspaceID(fromID, toID string) error {
	fromID = strings.TrimSpace(fromID)
	toID = strings.TrimSpace(toID)
	if fromID == "" || toID == "" {
		return errRebindIncomplete
	}
	if fromID == toID {
		return nil
	}
	r := loadRegistry()
	if !r.Has(fromID) {
		return errRebindIncomplete
	}
	if r.Has(toID) {
		return errRebindTargetInUse
	}
	next := workspace.Registry{
		Workspaces: make([]workspace.Workspace, 0, len(r.Workspaces)),
		ActiveID:   r.ActiveID,
		StartupID:  r.StartupID,
	}
	for _, w := range r.Workspaces {
		if w.ID == fromID {
			w.ID = toID
		}
		next.Workspaces = append(next.Workspaces, w)
	}
	if next.ActiveID == fromID {
		next.ActiveID = toID
	}
	if next.StartupID == fromID {
		next.StartupID = toID
	}
	// The INACTIVE workspaces keep their state in a per-id blob; the active one
	// keeps its state in the live keys, so it has no blob to move. Moving a blob
	// that is not there would be harmless, but reading first keeps the intent
	// legible.
	if blob := lsGet(wsBlobKey(fromID)); blob != "" {
		lsSet(wsBlobKey(toID), blob)
		lsRemove(wsBlobKey(fromID))
	}
	saveRegistry(next)
	return nil
}

// clearSyncMetaFor forgets the sync bookkeeping for a workspace id.
//
// It is called after a rebind because that metadata records a conversation with
// the server about a workspace the current account was never party to. Carrying
// it across would let a stale version number and hash decide the first exchange
// with the new workspace — the client would compare against the wrong history
// and could conclude it was already up to date.
//
// The CONFLICT backup is MOVED rather than dropped. It holds a local edit that
// lost a last-write-wins race — the user's data, and the one thing here that
// cannot be reconstructed from the server. Leaving it under the old id would
// have stranded it: every reader of that backup keys off the ACTIVE workspace's
// id, which after a rebind is the new one, so an untouched backup becomes
// unreachable from any surface that could restore it. Found by adversarial
// review, 2026-08-17.
func clearSyncMetaFor(workspaceID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	lsRemove(syncMetaKey(workspaceID))
}

// carryConflictBackup moves a pending conflict backup onto the new workspace id.
func carryConflictBackup(fromID, toID string) {
	fromID, toID = strings.TrimSpace(fromID), strings.TrimSpace(toID)
	if fromID == "" || toID == "" || fromID == toID {
		return
	}
	stashed := lsGet(syncConflictPrefix + fromID)
	if stashed == "" {
		return
	}
	// Never overwrite a backup already waiting on the target: that one is also
	// an unrecoverable local edit, and this is a repair, not a tidy-up.
	if lsGet(syncConflictPrefix+toID) == "" {
		lsSet(syncConflictPrefix+toID, stashed)
	}
	lsRemove(syncConflictPrefix + fromID)
}
