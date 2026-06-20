// Package history is the pure, diff-based change-history core behind CashFlux's
// audit log and undo/redo (C78 phase 1). It snapshots the dataset before and after
// a mutation, diffs the two into a minimal id-keyed ChangeSet (forward + inverse),
// and applies a ChangeSet forward or inverted. Because reversal is computed from
// the diff rather than hand-written, cascades (a transfer-pair delete, reassign-on-
// delete, cover-budget) reverse for free.
//
// It is generic over the dataset: a Snapshot is collection name → row id → row
// JSON, so the differ works across every Dataset collection without knowing their
// Go types. Pure Go, no syscall/js, no store/appstate import; unit-tested natively.
package history

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Op is the kind of change to a single row.
type Op string

const (
	OpAdd    Op = "add"
	OpUpdate Op = "update"
	OpDelete Op = "delete"
)

// Snapshot is a point-in-time view of the data: collection name → row id → the
// row's serialized JSON. Callers build it from store.Snapshot()/Load; this package
// never needs the concrete row types.
type Snapshot map[string]map[string]json.RawMessage

// Change is one row's transition. Before is nil for an add; After is nil for a
// delete; both are set (and differ) for an update.
type Change struct {
	Collection string          `json:"collection"`
	ID         string          `json:"id"`
	Op         Op              `json:"op"`
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
}

// ChangeSet is the diff of one mutation — a labelled, deterministic list of row
// changes. Applying it moves a snapshot forward; applying its inverse undoes it.
type ChangeSet struct {
	Label   string   `json:"label,omitempty"`
	Changes []Change `json:"changes"`
}

// IsEmpty reports whether the change set touches nothing (a no-op mutation).
func (cs ChangeSet) IsEmpty() bool { return len(cs.Changes) == 0 }

// Bytes is the change set's approximate serialized size (the sum of its before/
// after row bytes), used by Stack to bound memory.
func (cs ChangeSet) Bytes() int {
	n := 0
	for _, c := range cs.Changes {
		n += len(c.Before) + len(c.After)
	}
	return n
}

// Diff computes the change set that turns before into after: a row only in after
// is an add, only in before is a delete, in both with differing bytes is an update,
// and an unchanged row produces nothing. Changes are sorted by collection then id
// so the result is deterministic.
func Diff(before, after Snapshot) ChangeSet {
	var cs ChangeSet
	for _, coll := range unionKeys(before, after) {
		b, a := before[coll], after[coll]
		for _, id := range unionRowIDs(b, a) {
			bv, bok := b[id]
			av, aok := a[id]
			switch {
			case !bok && aok:
				cs.Changes = append(cs.Changes, Change{Collection: coll, ID: id, Op: OpAdd, After: clone(av)})
			case bok && !aok:
				cs.Changes = append(cs.Changes, Change{Collection: coll, ID: id, Op: OpDelete, Before: clone(bv)})
			case bok && aok && !bytes.Equal(bv, av):
				cs.Changes = append(cs.Changes, Change{Collection: coll, ID: id, Op: OpUpdate, Before: clone(bv), After: clone(av)})
			}
		}
	}
	return cs
}

// Invert returns the change set that undoes cs: adds become deletes, deletes become
// adds, and updates swap before/after.
func (cs ChangeSet) Invert() ChangeSet {
	out := ChangeSet{Label: cs.Label, Changes: make([]Change, len(cs.Changes))}
	for i, c := range cs.Changes {
		inv := Change{Collection: c.Collection, ID: c.ID, Before: c.After, After: c.Before}
		switch c.Op {
		case OpAdd:
			inv.Op = OpDelete
		case OpDelete:
			inv.Op = OpAdd
		default:
			inv.Op = OpUpdate
		}
		out.Changes[i] = inv
	}
	return out
}

// Apply returns a copy of s with cs applied: adds and updates write the After row,
// deletes remove the row. The input snapshot is not modified.
func (cs ChangeSet) Apply(s Snapshot) Snapshot {
	out := s.Clone()
	for _, c := range cs.Changes {
		switch c.Op {
		case OpDelete:
			if rows := out[c.Collection]; rows != nil {
				delete(rows, c.ID)
			}
		default: // add / update
			rows := out[c.Collection]
			if rows == nil {
				rows = map[string]json.RawMessage{}
				out[c.Collection] = rows
			}
			rows[c.ID] = clone(c.After)
		}
	}
	return out
}

// Clone returns a deep copy of the snapshot (maps and row bytes copied), so a copy
// can be mutated without aliasing the original.
func (s Snapshot) Clone() Snapshot {
	out := make(Snapshot, len(s))
	for coll, rows := range s {
		cp := make(map[string]json.RawMessage, len(rows))
		for id, raw := range rows {
			cp[id] = clone(raw)
		}
		out[coll] = cp
	}
	return out
}

func clone(b json.RawMessage) json.RawMessage {
	if b == nil {
		return nil
	}
	out := make(json.RawMessage, len(b))
	copy(out, b)
	return out
}

func unionKeys(a, b Snapshot) []string {
	seen := map[string]struct{}{}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	return sortedKeys(seen)
}

func unionRowIDs(a, b map[string]json.RawMessage) []string {
	seen := map[string]struct{}{}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	return sortedKeys(seen)
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
