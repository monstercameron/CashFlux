// SPDX-License-Identifier: MIT

package history

import (
	"encoding/json"
	"testing"
)

// cs builds a single-change change set for one row update with the given byte-ish
// before/after payloads.
func cs(coll, id, before, after string) ChangeSet {
	c := Change{Collection: coll, ID: id, Op: OpUpdate}
	if before != "" {
		c.Before = json.RawMessage(before)
	}
	if after != "" {
		c.After = json.RawMessage(after)
	}
	return ChangeSet{Changes: []Change{c}}
}

func TestStackUndoRedoCursor(t *testing.T) {
	s := NewStack(0)
	if s.CanUndo() || s.CanRedo() {
		t.Fatal("empty stack has nothing to undo/redo")
	}
	s.Push(cs("txn", "t1", `{"v":1}`, `{"v":2}`))
	s.Push(cs("txn", "t1", `{"v":2}`, `{"v":3}`))
	if !s.CanUndo() || s.CanRedo() {
		t.Fatal("after two pushes: can undo, cannot redo")
	}
	// Undo returns the inverse of the latest (v3 → v2).
	inv, ok := s.Undo()
	if !ok || string(inv.Changes[0].After) != `{"v":2}` {
		t.Errorf("undo inverse After = %s, want {\"v\":2}", inv.Changes[0].After)
	}
	if !s.CanRedo() {
		t.Error("after one undo, redo should be available")
	}
	// Redo returns the forward change again (v2 → v3).
	fwd, ok := s.Redo()
	if !ok || string(fwd.Changes[0].After) != `{"v":3}` {
		t.Errorf("redo forward After = %s, want {\"v\":3}", fwd.Changes[0].After)
	}
}

func TestStackPushDiscardsRedoTail(t *testing.T) {
	s := NewStack(0)
	s.Push(cs("txn", "t1", `{"v":1}`, `{"v":2}`))
	s.Push(cs("txn", "t1", `{"v":2}`, `{"v":3}`))
	s.Undo()                                      // now one redoable
	s.Push(cs("txn", "t1", `{"v":2}`, `{"v":9}`)) // diverge
	if s.CanRedo() {
		t.Error("pushing after an undo must discard the redo tail")
	}
	if s.Len() != 2 {
		t.Errorf("len = %d, want 2 (the discarded redo entry is gone)", s.Len())
	}
}

func TestStackEmptyPushIgnored(t *testing.T) {
	s := NewStack(0)
	s.Push(ChangeSet{})
	if s.Len() != 0 || s.CanUndo() {
		t.Error("an empty change set should not become an undo step")
	}
}

func TestStackByteCapDropsOldest(t *testing.T) {
	// Each entry ~ len(before)+len(after) bytes. Use ~10-byte payloads, cap small.
	s := NewStack(40)
	for i := 0; i < 5; i++ {
		s.Push(cs("txn", "t1", `{"vvvv":1}`, `{"vvvv":2}`)) // ~20 bytes each
	}
	// Cap 40 / ~20 per entry → at most 2 retained.
	if s.Len() > 2 {
		t.Errorf("len = %d, want <= 2 after byte-cap eviction", s.Len())
	}
	if !s.CanUndo() {
		t.Error("at least one entry should survive the cap")
	}
}

func TestStackCoalesceSameRow(t *testing.T) {
	s := NewStack(0)
	s.PushCoalescing(cs("txn", "t1", `{"v":1}`, `{"v":2}`))
	s.PushCoalescing(cs("txn", "t1", `{"v":2}`, `{"v":3}`)) // same row → merges
	if s.Len() != 1 {
		t.Fatalf("len = %d, want 1 (two same-row edits coalesce)", s.Len())
	}
	inv, _ := s.Undo()
	// One undo reverts the whole burst: After is the ORIGINAL before (v1).
	if string(inv.Changes[0].After) != `{"v":1}` {
		t.Errorf("coalesced undo After = %s, want {\"v\":1} (original before)", inv.Changes[0].After)
	}
}

func TestStackCoalesceDifferentRowDoesNotMerge(t *testing.T) {
	s := NewStack(0)
	s.PushCoalescing(cs("txn", "t1", `{"v":1}`, `{"v":2}`))
	s.PushCoalescing(cs("txn", "t2", `{"v":1}`, `{"v":2}`)) // different row → separate
	if s.Len() != 2 {
		t.Errorf("len = %d, want 2 (different rows don't coalesce)", s.Len())
	}
}
