package history

// Stack is a bounded undo/redo stack of change sets. Entries before the cursor are
// applied (undoable); entries at/after the cursor were undone (redoable). A new
// Push discards the redoable tail (you can't redo after diverging) and enforces a
// byte cap by dropping the oldest entries — matching the autosave's quota-aware,
// drop-oldest policy. The zero value is unusable; construct with NewStack.
type Stack struct {
	entries []ChangeSet
	cursor  int // entries[:cursor] are undoable; entries[cursor:] are redoable
	byteCap int // 0 = unbounded
	bytes   int
}

// NewStack returns an empty stack bounded to byteCap bytes of stored before/after
// row data (0 = unbounded).
func NewStack(byteCap int) *Stack { return &Stack{byteCap: byteCap} }

// Push records a new change set as the latest undoable action, dropping any
// redoable tail. Empty change sets are ignored (a no-op mutation isn't an undo
// step).
func (s *Stack) Push(cs ChangeSet) {
	if cs.IsEmpty() {
		return
	}
	s.dropRedoTail()
	s.entries = append(s.entries, cs)
	s.cursor++
	s.bytes += cs.Bytes()
	s.enforceCap()
}

// PushCoalescing is like Push but merges into the previous entry when both touch
// exactly the same single row (e.g. rapid edits to one field), so a burst collapses
// into one undo step. It only coalesces at the tip (no redoable tail), preserving
// the original Before so one undo reverts the whole burst.
func (s *Stack) PushCoalescing(cs ChangeSet) {
	if cs.IsEmpty() {
		return
	}
	if s.cursor == len(s.entries) && s.cursor > 0 {
		top := s.entries[s.cursor-1]
		if a, b, ok := singleSameRow(top, cs); ok {
			merged := ChangeSet{Label: cs.Label, Changes: []Change{coalesce(a, b)}}
			s.bytes += merged.Bytes() - top.Bytes()
			s.entries[s.cursor-1] = merged
			s.enforceCap()
			return
		}
	}
	s.Push(cs)
}

// CanUndo reports whether there is an action to undo.
func (s *Stack) CanUndo() bool { return s.cursor > 0 }

// CanRedo reports whether there is an undone action to redo.
func (s *Stack) CanRedo() bool { return s.cursor < len(s.entries) }

// Undo moves the cursor back one and returns the inverse change set to apply, plus
// true. It returns false when there's nothing to undo.
func (s *Stack) Undo() (ChangeSet, bool) {
	if !s.CanUndo() {
		return ChangeSet{}, false
	}
	s.cursor--
	return s.entries[s.cursor].Invert(), true
}

// Redo moves the cursor forward one and returns the forward change set to re-apply,
// plus true. It returns false when there's nothing to redo.
func (s *Stack) Redo() (ChangeSet, bool) {
	if !s.CanRedo() {
		return ChangeSet{}, false
	}
	cs := s.entries[s.cursor]
	s.cursor++
	return cs, true
}

// Len returns the number of stored entries (undoable + redoable).
func (s *Stack) Len() int { return len(s.entries) }

func (s *Stack) dropRedoTail() {
	for i := s.cursor; i < len(s.entries); i++ {
		s.bytes -= s.entries[i].Bytes()
	}
	s.entries = s.entries[:s.cursor]
}

// enforceCap drops the oldest entries until within byteCap, keeping at least one.
func (s *Stack) enforceCap() {
	if s.byteCap <= 0 {
		return
	}
	for s.bytes > s.byteCap && len(s.entries) > 1 {
		s.bytes -= s.entries[0].Bytes()
		s.entries = s.entries[1:]
		if s.cursor > 0 {
			s.cursor--
		}
	}
}

// singleSameRow reports whether both change sets are a single change to the same
// (collection, id), returning the two changes.
func singleSameRow(a, b ChangeSet) (Change, Change, bool) {
	if len(a.Changes) != 1 || len(b.Changes) != 1 {
		return Change{}, Change{}, false
	}
	ca, cb := a.Changes[0], b.Changes[0]
	if ca.Collection != cb.Collection || ca.ID != cb.ID {
		return Change{}, Change{}, false
	}
	return ca, cb, true
}

// coalesce merges two changes to the same row into one spanning prev.Before →
// next.After, collapsing the op (an add followed by an edit is still an add; an
// edit ending in a delete is a delete).
func coalesce(prev, next Change) Change {
	out := Change{Collection: prev.Collection, ID: prev.ID, Before: prev.Before, After: next.After}
	switch {
	case prev.Op == OpAdd && next.Op == OpDelete:
		// Added then removed within the burst — nets to nothing, but keep a delete
		// of the pre-existing (nil) row so applying is a safe no-op either way.
		out.Op = OpDelete
	case prev.Op == OpAdd:
		out.Op = OpAdd
	case next.Op == OpDelete:
		out.Op = OpDelete
	default:
		out.Op = OpUpdate
	}
	return out
}
