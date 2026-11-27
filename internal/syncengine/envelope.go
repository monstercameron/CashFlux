// SPDX-License-Identifier: MIT

// Package syncengine implements CashFlux's pure, platform-independent v2
// BlobSync state machine, record-level change batching, conflict detection,
// durable checkpoint creation, and serialized push/pull orchestration.
//
// This package has zero syscall/js dependencies and unit-tests on native Go.
package syncengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/store"
)

// BatchFormatVersion is the wire schema version of a decrypted v2 ChangeBatch.
const BatchFormatVersion = 1

// BatchKind distinguishes incremental record-change batches from full dataset
// snapshot replacements inside the v2 append-only change log.
type BatchKind string

const (
	BatchKindRecords  BatchKind = "records"
	BatchKindSnapshot BatchKind = "snapshot"
)

// RecordKey uniquely identifies one row in the CashFlux SQLite schema.
type RecordKey struct {
	Table string `json:"table"`
	ID    string `json:"id"`
}

// String formats the key for deterministic sorting and diagnostics.
func (k RecordKey) String() string { return k.Table + "/" + k.ID }

// ChangeBatch is the decrypted payload carried inside one v2 BlobChange.Blob.
// Every batch is encrypted client-side before leaving the browser; the server
// only stores opaque ciphertext and assigns a monotonic gap-free sequence.
type ChangeBatch struct {
	Version      int                  `json:"v"`
	ChangeID     string               `json:"changeId"`
	WorkspaceID  string               `json:"workspaceId"`
	DeviceID     string               `json:"deviceId"`
	BaseSeq      int64                `json:"baseSeq"`
	Kind         BatchKind            `json:"kind"`
	Changes      []store.RecordChange `json:"changes,omitempty"`
	SnapshotJSON json.RawMessage      `json:"snapshotJson,omitempty"`
	CreatedAt    time.Time            `json:"createdAt"`
}

// Validate enforces structural invariants before encoding or applying a batch.
func (b ChangeBatch) Validate() error {
	if b.Version != BatchFormatVersion {
		return fmt.Errorf("syncengine: unsupported batch version %d", b.Version)
	}
	if strings.TrimSpace(b.ChangeID) == "" {
		return errors.New("syncengine: batch changeId is required")
	}
	if strings.TrimSpace(b.WorkspaceID) == "" {
		return errors.New("syncengine: batch workspaceId is required")
	}
	if strings.TrimSpace(b.DeviceID) == "" {
		return errors.New("syncengine: batch deviceId is required")
	}
	if b.BaseSeq < 0 {
		return errors.New("syncengine: batch baseSeq cannot be negative")
	}
	switch b.Kind {
	case BatchKindRecords:
		if len(b.Changes) == 0 {
			return errors.New("syncengine: record batch requires at least one change")
		}
	case BatchKindSnapshot:
		if len(b.SnapshotJSON) == 0 || !json.Valid(b.SnapshotJSON) {
			return errors.New("syncengine: snapshot batch requires valid JSON")
		}
	default:
		return fmt.Errorf("syncengine: unknown batch kind %q", b.Kind)
	}
	return nil
}

// Encode serializes a validated ChangeBatch into canonical JSON bytes ready
// for client-side encryption.
func (b ChangeBatch) Encode() ([]byte, error) {
	if b.Version == 0 {
		b.Version = BatchFormatVersion
	}
	if b.Kind == BatchKindRecords && len(b.Changes) > 0 {
		b.Changes = store.CoalesceChanges(b.Changes)
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("syncengine: encode batch: %w", err)
	}
	return raw, nil
}

// DecodeChangeBatch parses and validates a decrypted v2 ChangeBatch payload.
func DecodeChangeBatch(raw []byte) (ChangeBatch, error) {
	var b ChangeBatch
	if err := json.Unmarshal(raw, &b); err != nil {
		return ChangeBatch{}, fmt.Errorf("syncengine: decode batch: %w", err)
	}
	if err := b.Validate(); err != nil {
		return ChangeBatch{}, err
	}
	return b, nil
}

// TouchedKeys returns the sorted, deduplicated set of record keys touched by a
// slice of RecordChanges.
func TouchedKeys(changes []store.RecordChange) []RecordKey {
	if len(changes) == 0 {
		return nil
	}
	seen := make(map[RecordKey]struct{}, len(changes))
	out := make([]RecordKey, 0, len(changes))
	for _, c := range changes {
		k := RecordKey{Table: c.Table, ID: c.ID}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
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
