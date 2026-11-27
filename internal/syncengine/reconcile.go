// SPDX-License-Identifier: MIT

package syncengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/datasetguard"
	"github.com/monstercameron/CashFlux/internal/store"
)

// MergePreference decides which side wins when both devices modified the exact
// same record offline during an explicit conflict merge.
type MergePreference string

const (
	PreferLocal  MergePreference = "local"
	PreferServer MergePreference = "server"
)

// ReconcileResult is the pure outcome of comparing pending local changes
// against newly pulled remote batches.
type ReconcileResult struct {
	// HasConflict is true when remote and local changes cannot be merged
	// automatically (either overlapping record keys or a full snapshot crossing
	// unpushed local edits).
	HasConflict bool
	// CollidingKeys lists the sorted (table, id) pairs touched by both sides.
	CollidingKeys []RecordKey
	// ApplySnapshotJSON is non-empty when remote changes include a full
	// snapshot replacement that can be fast-forwarded cleanly.
	ApplySnapshotJSON json.RawMessage
	// ApplyRecords contains the ordered remote record transitions to replay
	// live in place (after ApplySnapshotJSON, if any).
	ApplyRecords []store.RecordChange
}

// ReconcileBatches compares unpushed local changes against decrypted remote
// batches (already filtered to exclude this device's own acked change IDs).
// Non-overlapping record edits auto-merge cleanly; overlapping record edits or
// snapshot crossings produce a deterministic conflict.
func ReconcileBatches(localPending []store.RecordChange, localHasSnapshot bool, remote []ChangeBatch) ReconcileResult {
	if len(remote) == 0 {
		return ReconcileResult{}
	}
	localDirty := len(localPending) > 0 || localHasSnapshot
	localKeys := make(map[RecordKey]struct{}, len(localPending))
	for _, c := range localPending {
		localKeys[RecordKey{Table: c.Table, ID: c.ID}] = struct{}{}
	}

	var res ReconcileResult
	collidingSet := make(map[RecordKey]struct{})
	lastSnapIdx := -1
	for i, batch := range remote {
		if batch.Kind == BatchKindSnapshot {
			lastSnapIdx = i
			if localDirty {
				res.HasConflict = true
			}
		}
	}

	if res.HasConflict {
		// Record any overlapping record keys for conflict diagnostics too.
		for _, batch := range remote {
			for _, c := range batch.Changes {
				k := RecordKey{Table: c.Table, ID: c.ID}
				if _, ok := localKeys[k]; ok {
					collidingSet[k] = struct{}{}
				}
			}
		}
		res.CollidingKeys = sortedRecordKeys(collidingSet)
		return res
	}

	// If local has a pending full snapshot and any remote change arrived,
	// branch into a conflict so neither side's edits are silently lost.
	if localHasSnapshot {
		res.HasConflict = true
		return res
	}

	// Fast-forward from the latest remote snapshot if one exists (local is clean).
	startIdx := 0
	if lastSnapIdx >= 0 {
		res.ApplySnapshotJSON = remote[lastSnapIdx].SnapshotJSON
		startIdx = lastSnapIdx + 1
	}

	var toApply []store.RecordChange
	for i := startIdx; i < len(remote); i++ {
		batch := remote[i]
		for _, c := range batch.Changes {
			k := RecordKey{Table: c.Table, ID: c.ID}
			if _, ok := localKeys[k]; ok {
				collidingSet[k] = struct{}{}
			}
			toApply = append(toApply, c)
		}
	}

	if len(collidingSet) > 0 {
		res.HasConflict = true
		res.CollidingKeys = sortedRecordKeys(collidingSet)
		return res
	}
	res.ApplyRecords = toApply
	return res
}

func sortedRecordKeys(set map[RecordKey]struct{}) []RecordKey {
	if len(set) == 0 {
		return nil
	}
	out := make([]RecordKey, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Table != out[j].Table {
			return out[i].Table < out[j].Table
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// CheckDestructiveGuard runs the client-side destructive-write guard on
// decrypted plaintext snapshots. Running this on the client closes the gap
// where server-side inspection cannot read SEC6 ciphertext envelopes.
func CheckDestructiveGuard(prevJSON, nextJSON []byte, force bool) error {
	if force || len(bytes.TrimSpace(prevJSON)) == 0 || len(bytes.TrimSpace(nextJSON)) == 0 {
		return nil
	}
	prevCounts, okPrev := datasetguard.Inspect(prevJSON)
	nextCounts, okNext := datasetguard.Inspect(nextJSON)
	if !okPrev || !okNext {
		return nil
	}
	verdict := datasetguard.Check(prevCounts, nextCounts, datasetguard.DefaultPolicy)
	if verdict.Destructive {
		return fmt.Errorf("syncengine: %s", verdict.Reason)
	}
	return nil
}

// MergeSummary describes the outcome of a user-confirmed conflict merge.
type MergeSummary struct {
	LocalApplied  int
	ServerKept    int
	Collisions    int
	Preference    MergePreference
	MergedRecords int
}

func loadJSONIntoStore(db *store.SQLiteStore, raw []byte) error {
	ds, err := store.Import(raw)
	if err != nil {
		return err
	}
	return db.Load(ds)
}

func exportJSONFromStore(db *store.SQLiteStore) ([]byte, error) {
	ds, err := db.Snapshot()
	if err != nil {
		return nil, err
	}
	return store.Export(ds)
}

// MergeDatasets constructs a referentially intact merged dataset from localJSON
// and serverJSON using an in-memory SQLite store.
//
// When localPending record transitions are available, the merge loads serverJSON
// and replays all non-conflicting local transitions on top, resolving colliding
// keys according to prefer. When localPending is empty (e.g., two full snapshots
// diverged across a truncated log), it merges entity collections cleanly by ID.
func MergeDatasets(localJSON, serverJSON []byte, localPending []store.RecordChange, colliding []RecordKey, prefer MergePreference) ([]byte, MergeSummary, error) {
	if prefer != PreferLocal && prefer != PreferServer {
		prefer = PreferLocal
	}
	summary := MergeSummary{Preference: prefer, Collisions: len(colliding)}

	if len(localPending) > 0 {
		collidingSet := make(map[RecordKey]struct{}, len(colliding))
		for _, k := range colliding {
			collidingSet[k] = struct{}{}
		}
		db, err := store.NewMemory()
		if err != nil {
			return nil, summary, fmt.Errorf("syncengine: open merge store: %w", err)
		}
		defer db.Close()
		if err := loadJSONIntoStore(db, serverJSON); err != nil {
			return nil, summary, fmt.Errorf("syncengine: import server dataset for merge: %w", err)
		}
		var toReplay []store.RecordChange
		for _, c := range store.CoalesceChanges(localPending) {
			k := RecordKey{Table: c.Table, ID: c.ID}
			if _, isCollision := collidingSet[k]; isCollision {
				if prefer == PreferServer {
					summary.ServerKept++
					continue
				}
			}
			toReplay = append(toReplay, c)
			summary.LocalApplied++
		}
		if err := db.ReplayChanges(toReplay); err != nil {
			return nil, summary, fmt.Errorf("syncengine: replay local changes onto server: %w", err)
		}
		merged, err := exportJSONFromStore(db)
		if err != nil {
			return nil, summary, fmt.Errorf("syncengine: export merged dataset: %w", err)
		}
		summary.MergedRecords = len(toReplay)
		return merged, summary, nil
	}

	// Snapshot-vs-snapshot fallback: merge typed Dataset collections by ID and
	// validate round-trip through SQLiteStore.
	localDS, err := store.Import(localJSON)
	if err != nil {
		return nil, summary, fmt.Errorf("syncengine: import local dataset: %w", err)
	}
	serverDS, err := store.Import(serverJSON)
	if err != nil {
		return nil, summary, fmt.Errorf("syncengine: import server dataset: %w", err)
	}
	base, overlay := serverDS, localDS
	if prefer == PreferServer {
		base, overlay = localDS, serverDS
	}
	mergedDS := mergeTypedDatasetsFull(base, overlay)
	raw, err := store.Export(mergedDS)
	if err != nil {
		return nil, summary, fmt.Errorf("syncengine: export merged snapshot: %w", err)
	}
	// Validate via SQLite round-trip so foreign/schema invariants hold.
	db, err := store.NewMemory()
	if err != nil {
		return nil, summary, fmt.Errorf("syncengine: open validation store: %w", err)
	}
	defer db.Close()
	if err := loadJSONIntoStore(db, raw); err != nil {
		return nil, summary, fmt.Errorf("syncengine: validate merged snapshot: %w", err)
	}
	out, err := exportJSONFromStore(db)
	if err != nil {
		return nil, summary, fmt.Errorf("syncengine: export validated merge: %w", err)
	}
	return out, summary, nil
}


