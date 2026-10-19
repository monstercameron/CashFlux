// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncstate"
)

// Which account does the dataset currently loaded on this device belong to?
//
// Until now nothing recorded the answer. The dataset lives under a canonical
// key bound to a WORKSPACE, and the workspace registry is device-global, so two
// accounts sharing one browser shared one dataset. Signing out deliberately
// keeps that dataset — unpushed work is the user's, and signing out is not a
// request to discard it — which means the next account inherits whatever the
// previous one left loaded.
//
// On 2026-08-19 that cost a household three weeks of records: the inherited copy
// looked newer than the arriving account's server snapshot, won the
// last-write-wins comparison, and was pushed over their real data. The queue had
// been taught about identity (C696); the DATASET never was.
//
// So the dataset gets an owner, and two rules follow from it:
//
//   - It is never pushed under an account that does not own it.
//   - It is never allowed to outrank that account's server copy on a pull.

// datasetOwnerKey records the account the loaded dataset belongs to. Stored
// beside the dataset rather than inside it: it describes which CONNECTION the
// data arrived on, not anything about the household, and putting it in the
// dataset would sync it to the server and make it another thing to conflict.
const datasetOwnerKey = "cashflux:dataset-owner"

// datasetOwner returns the account the loaded dataset belongs to, or "" when
// nothing has claimed it (a fresh install, or a device that predates this).
func datasetOwner() syncstate.DatasetOwner {
	owner, legacy := syncstate.ParseDatasetOwner(lsGet(datasetOwnerKey), currentDatasetServerID())
	if legacy && owner.ServerID != "" {
		if encoded, err := json.Marshal(owner); err == nil {
			lsSet(datasetOwnerKey, string(encoded))
		}
	}
	return owner
}

// claimDatasetForCurrentUser records the signed-in account as the owner of
// whatever is loaded now.
//
// Called at exactly the two moments the data provably belongs to that account:
// after their server copy has been applied to this device, and after this
// device's copy has been accepted by the server as theirs. Claiming anywhere
// else would launder an inherited dataset into a legitimate one, which is the
// whole failure being prevented.
func claimDatasetForCurrentUser() {
	owner := currentDatasetOwner()
	if owner.UserID == "" || owner.ServerID == "" {
		// Unknown identity claims nothing. See syncstate.DatasetForeign: an
		// unknown identity must never be treated as authoritative, in either
		// direction.
		return
	}
	if datasetOwner() == owner {
		return
	}
	encoded, err := json.Marshal(owner)
	if err != nil {
		return
	}
	lsSet(datasetOwnerKey, string(encoded))
}

// releaseDatasetOwner forgets the claim. Used when the local dataset is
// deliberately discarded (a wipe), so the next thing loaded starts unowned and
// therefore adoptable rather than foreign.
func releaseDatasetOwner() { lsRemove(datasetOwnerKey) }

// datasetIsForeign reports whether the loaded dataset belongs to a different
// account than the one signed in now.
func datasetIsForeign() bool {
	return syncstate.DatasetOwnerForeign(datasetOwner(), currentDatasetOwner())
}

// foreignDatasetStashKey is where an inherited dataset is parked when the
// arriving account's copy replaces it.
//
// Parked, not deleted. It is somebody's unpushed work — possibly the only copy —
// and the failure this whole change exists to prevent was CAUSED by treating one
// account's data as disposable in the presence of another. Keyed by the owning
// account so two of them cannot overwrite each other in the stash either.
func foreignDatasetStashKey(owner syncstate.DatasetOwner) string {
	return "cashflux:dataset-stash:" + owner.ServerID + ":" + owner.UserID
}

// stashForeignDataset parks the loaded dataset under its owner's key before
// something else replaces it. A no-op when the dataset is unowned or already
// belongs to the signed-in account.
func stashForeignDataset() error {
	owner := datasetOwner()
	if owner.UserID == "" || !datasetIsForeign() {
		return nil
	}
	raw, expected, err := materializeLocalDataset()
	if err != nil {
		quarantineDataset(expected[datasetStoreKey], "This account's saved changes could not be recovered: "+err.Error())
		return fmt.Errorf("preserve previous account's dataset: %w", err)
	}
	if raw == "" {
		return nil
	}
	observedOwner, _ := syncstate.ParseDatasetOwner(expected[datasetOwnerKey], owner.ServerID)
	if observedOwner != owner {
		return fmt.Errorf("previous account changed while preparing its recovery copy")
	}
	key := foreignDatasetStashKey(owner)
	previous, _, err := browserstore.GetSecure(key)
	if err != nil {
		return fmt.Errorf("read previous account's recovery copy: %w", err)
	}
	expected[key] = previous
	values := make(map[string]string, len(expected))
	for key, raw := range expected {
		values[key] = raw
	}
	values[key] = raw
	if err := awaitSecureAction(func(done func(error)) {
		browserstore.SetSecureBatch(values, expected, done)
	}); err != nil {
		return fmt.Errorf("save previous account's recovery copy: %w", err)
	}
	if app := appstate.Default; app != nil {
		app.Log().Warn("parked another account's dataset before replacing it",
			"owner_server", owner.ServerID, "owner_user", owner.UserID, "bytes", len(raw))
	}
	return nil
}

// materializeLocalDataset reads the authenticated persisted checkpoint and its
// entire journal, never the live store (which may already belong to a new user).
// Callers must compare all returned preconditions in their durable write. This
// function can await legacy decryption and must run outside a JS event callback.
func materializeLocalDataset() (string, map[string]string, error) {
	expected := make(map[string]string)
	snapshot, err := readDurableDatasetSnapshot()
	if err != nil {
		return "", expected, fmt.Errorf("read saved dataset snapshot: %w", err)
	}
	for _, key := range []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey} {
		raw, _, err := snapshot.Read(key)
		if err != nil {
			return "", expected, fmt.Errorf("read saved dataset metadata: %w", err)
		}
		expected[key] = raw
	}
	checkpoint := expected[datasetStoreKey]
	if checkpoint == "" && expected[datasetHeadKey] == "" {
		return "", expected, nil
	}
	records, err := readDatasetRecords(checkpoint, expected[datasetHeadKey], expected[datasetGenKey], snapshot.workspace, snapshot.Read)
	if err != nil {
		return "", expected, err
	}
	if cryptobox.IsEnvelope([]byte(checkpoint)) {
		if activePasscode == "" {
			return "", expected, errSyncDatasetLocked
		}
		plain, err := decryptEnvelopeSync([]byte(checkpoint), activePasscode)
		if err != nil {
			return "", expected, fmt.Errorf("open legacy dataset: %w", err)
		}
		checkpoint = string(plain)
	}
	portable, err := portableDatasetFromRecords(checkpoint, records)
	if err != nil {
		return "", expected, err
	}
	// A checkpoint without journal entries still needs schema/content validation.
	if _, err := store.Import([]byte(portable)); err != nil {
		return "", expected, fmt.Errorf("validate saved dataset: %w", err)
	}
	return portable, expected, nil
}
