// SPDX-License-Identifier: MIT

package changes_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/changes"
)

// seqID returns a deterministic valid UUID v4 for the nth test batch.
func seqID(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

// batchAt builds a valid batch from device at (wall, logical).
func batchAt(seq int, device string, wall int64, logical uint32, ops ...changes.Op) changes.Batch {
	return changes.Batch{
		ChangeID:      seqID(seq),
		Scope:         changes.Scope{ServerID: "srv-1", AccountID: "acct-1"},
		WorkspaceID:   "ws-1",
		DeviceID:      device,
		HLC:           changes.HLC{WallMillis: wall, Logical: logical},
		SchemaVersion: changes.CurrentSchemaVersion,
		Ops:           ops,
	}
}

// writeOp builds a field-write op from raw JSON literals.
func writeOp(table, id string, fields map[string]string) changes.Op {
	op := changes.Op{Table: table, RecordID: id, Fields: make(map[string]json.RawMessage, len(fields))}
	for k, v := range fields {
		op.Fields[k] = json.RawMessage(v)
	}
	return op
}

// removeOp builds a field-removal op.
func removeOp(table, id string, fields ...string) changes.Op {
	return changes.Op{Table: table, RecordID: id, Removed: fields}
}

// deleteOp builds a record-delete (tombstone) op.
func deleteOp(table, id string) changes.Op {
	return changes.Op{Table: table, RecordID: id, Deleted: true}
}

func mustApply(t *testing.T, s *changes.State, batches ...changes.Batch) {
	t.Helper()
	for _, b := range batches {
		if err := s.Apply(b); err != nil {
			t.Fatalf("Apply(%s): %v", b.ChangeID, err)
		}
	}
}

func mustMaterialize(t *testing.T, s *changes.State, table, id, want string) {
	t.Helper()
	got, ok := s.Materialize(table, id)
	if !ok {
		t.Fatalf("Materialize(%s/%s) = not found, want %s", table, id, want)
	}
	if string(got) != want {
		t.Fatalf("Materialize(%s/%s) = %s, want %s", table, id, got, want)
	}
}

func mustNotExist(t *testing.T, s *changes.State, table, id string) {
	t.Helper()
	if got, ok := s.Materialize(table, id); ok {
		t.Fatalf("Materialize(%s/%s) = %s, want not found", table, id, got)
	}
}

// fakePhysical is a hand-driven physical clock replaying fixed readings.
type fakePhysical struct {
	readings []int64
	idx      int
	now      int64
}

func (f *fakePhysical) Next() int64 {
	if f.idx < len(f.readings) {
		f.now = f.readings[f.idx]
		f.idx++
	}
	return f.now
}

func (f *fakePhysical) Set(v int64) { f.now = v; f.readings = nil; f.idx = 0 }
