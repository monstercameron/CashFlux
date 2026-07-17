// SPDX-License-Identifier: MIT

// Package checkpoint manages the pre-operation safety-checkpoint ring (#55):
// before a risky bulk operation (an import, apply-all-rules, bulk delete /
// recategorize, cover-all, an allocation apply) the app snapshots the whole
// dataset into a small capped ring so the operation can be rolled back in one
// click even after the session's undo stack is gone.
//
// This package is pure bookkeeping over the ring's INDEX — the snapshot blobs
// themselves are stored by the caller (browser IndexedDB on wasm), keyed by
// checkpoint ID. Keeping index policy here makes the drop-oldest and JSON
// round-trip behavior natively testable.
package checkpoint

import (
	"encoding/json"
	"time"
)

// Checkpoint describes one saved pre-operation snapshot. The dataset blob is
// stored separately under the checkpoint's ID; Size records the blob's byte
// length for display.
type Checkpoint struct {
	ID    string    `json:"id"`
	At    time.Time `json:"at"`
	Label string    `json:"label"`
	Size  int       `json:"size"`
}

// MaxEntries caps the ring: five checkpoints cover a realistic string of risky
// operations without letting whole-dataset blobs accumulate in browser storage.
const MaxEntries = 5

// Push appends cp to the index (oldest-first order) and enforces the cap,
// dropping the oldest entries beyond max. It returns the kept index and the
// dropped entries so the caller can delete their stored blobs. The input slice
// is never mutated; max values below 1 are treated as 1.
func Push(index []Checkpoint, cp Checkpoint, max int) (kept, dropped []Checkpoint) {
	if max < 1 {
		max = 1
	}
	all := make([]Checkpoint, 0, len(index)+1)
	all = append(all, index...)
	all = append(all, cp)
	if over := len(all) - max; over > 0 {
		dropped = all[:over:over]
		kept = all[over:]
		return kept, dropped
	}
	return all, nil
}

// Remove deletes the checkpoint with the given ID from the index, returning
// the new index and whether it was found. The input slice is never mutated.
func Remove(index []Checkpoint, id string) ([]Checkpoint, bool) {
	out := make([]Checkpoint, 0, len(index))
	found := false
	for _, c := range index {
		if c.ID == id {
			found = true
			continue
		}
		out = append(out, c)
	}
	return out, found
}

// Find returns the checkpoint with the given ID and whether it exists.
func Find(index []Checkpoint, id string) (Checkpoint, bool) {
	for _, c := range index {
		if c.ID == id {
			return c, true
		}
	}
	return Checkpoint{}, false
}

// EncodeIndex serializes the index for storage ("" for an empty index).
func EncodeIndex(index []Checkpoint) string {
	if len(index) == 0 {
		return ""
	}
	b, err := json.Marshal(index)
	if err != nil {
		return ""
	}
	return string(b)
}

// DecodeIndex parses a stored index; malformed or empty input yields nil so a
// corrupt index degrades to "no checkpoints" instead of an error state.
func DecodeIndex(raw string) []Checkpoint {
	if raw == "" {
		return nil
	}
	var out []Checkpoint
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
