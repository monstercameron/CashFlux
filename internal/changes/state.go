// SPDX-License-Identifier: MIT

package changes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// DefaultSeenBound is the default maximum number of ChangeIDs a State
// remembers in its idempotency seen-set. When the bound is reached the
// oldest IDs are evicted first-in-first-out. Eviction does not break
// idempotence for honestly generated batches: re-applying an evicted batch
// is still a no-op because every one of its field stamps compares equal to
// (not greater than) the stored stamps. The seen-set's remaining job is to
// make exact duplicates cheap and to absorb a ChangeID collision for as long
// as the ID is retained; ChangeIDs are random UUID v4s, so collisions are
// not expected outside a broken or hostile producer.
const DefaultSeenBound = 100_000

// recordKey identifies one record across all tables.
type recordKey struct {
	table string
	id    string
}

// fieldState is one field's current value (or removal) and the stamp that
// won it.
type fieldState struct {
	value   json.RawMessage
	stamp   Stamp
	deleted bool
}

// recordState is the merge state of one record: per-field winners, the
// tombstone stamp of the newest delete, and the newest non-delete stamp
// (which keeps an empty "{}" document distinguishable from a deleted one).
type recordState struct {
	fields    map[string]fieldState
	tombstone Stamp
	hasTomb   bool
	alive     Stamp
	hasAlive  bool
}

// StateConfig configures a State. The zero value is an unbound state at the
// current schema version with the default seen bound.
type StateConfig struct {
	// Scope, when non-zero, binds the state to one (server, account):
	// batches from any other scope are rejected with ErrScopeMismatch so
	// data can never be mixed across scopes.
	Scope Scope
	// WorkspaceID, when non-empty, additionally binds the state to one
	// workspace within the scope.
	WorkspaceID string
	// SupportedSchemaVersion overrides the newest batch schema Apply
	// accepts. Zero means MaxSupportedSchemaVersion.
	SupportedSchemaVersion int
	// SeenBound caps the idempotency seen-set (see DefaultSeenBound).
	// Zero means the default; a negative value disables eviction entirely
	// (unbounded memory — useful only in tests).
	SeenBound int
}

// State is the merged result of every batch applied to it. It is safe for
// concurrent use. The zero value is not usable; construct with NewState.
type State struct {
	mu         sync.RWMutex
	scope      Scope
	workspace  string
	supported  int
	seenBound  int
	records    map[recordKey]*recordState
	seen       map[string]struct{}
	seenOrder  []string
	seenHead   int
	applied    uint64
	duplicates uint64
}

// NewState returns an empty, unbound State.
func NewState() *State { return NewStateWithConfig(StateConfig{}) }

// NewScopedState returns an empty State bound to one scope and workspace.
func NewScopedState(scope Scope, workspaceID string) *State {
	return NewStateWithConfig(StateConfig{Scope: scope, WorkspaceID: workspaceID})
}

// NewStateWithConfig returns an empty State configured by cfg.
func NewStateWithConfig(cfg StateConfig) *State {
	supported := cfg.SupportedSchemaVersion
	if supported == 0 {
		supported = MaxSupportedSchemaVersion
	}
	bound := cfg.SeenBound
	if bound == 0 {
		bound = DefaultSeenBound
	}
	return &State{
		scope:     cfg.Scope,
		workspace: cfg.WorkspaceID,
		supported: supported,
		seenBound: bound,
		records:   make(map[recordKey]*recordState),
		seen:      make(map[string]struct{}),
	}
}

// SupportedSchemaVersion returns the newest batch schema this state accepts.
func (s *State) SupportedSchemaVersion() int { return s.supported }

// Apply merges one batch into the state.
//
// The batch is validated in full before anything is written; a failed batch
// (invalid, newer schema, wrong scope) applies nothing at all and is not
// recorded in the seen-set, so it can be retried after an upgrade. Applying
// the same ChangeID twice is a no-op. Per op, with the batch's Stamp:
//
//   - A field write replaces the stored value only if its stamp is strictly
//     greater than the field's stored stamp and strictly greater than the
//     record tombstone. A write newer than the tombstone revives a deleted
//     record carrying only fields newer than the tombstone.
//   - A field removal is stored as a per-field tombstone under the same rule,
//     so older writes that arrive later cannot resurrect the field.
//   - A record delete advances the tombstone (only if strictly greater) and
//     drops every field whose stamp is not greater than the tombstone. A
//     delete whose stamp ties with a field write wins the tie.
//
// These rules make Apply idempotent, commutative and associative over any
// set of batches. Apply never mutates its argument and deep-copies every
// value it keeps.
func (s *State) Apply(b Batch) error {
	if err := b.validate(s.supported); err != nil {
		return err
	}
	if err := s.checkScope(b); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.seen[b.ChangeID]; dup {
		s.duplicates++
		return nil
	}
	stamp := b.Stamp()
	for i := range b.Ops {
		s.applyOp(&b.Ops[i], stamp)
	}
	s.remember(b.ChangeID)
	s.applied++
	return nil
}

// ApplyAll applies batches in order, stopping at the first error. Batches
// before the failing one stay applied (each batch is atomic; the slice is
// not). It returns how many batches were applied.
func (s *State) ApplyAll(batches []Batch) (int, error) {
	for i := range batches {
		if err := s.Apply(batches[i]); err != nil {
			return i, err
		}
	}
	return len(batches), nil
}

// checkScope enforces the state's scope binding. It runs before the write
// lock on immutable config only.
func (s *State) checkScope(b Batch) error {
	if !s.scope.IsZero() && b.Scope != s.scope {
		return fmt.Errorf("changes: batch %s is for scope %s, state is bound to %s: %w",
			b.ChangeID, b.Scope, s.scope, ErrScopeMismatch)
	}
	if s.workspace != "" && b.WorkspaceID != s.workspace {
		return fmt.Errorf("changes: batch %s is for workspace %q, state is bound to %q: %w",
			b.ChangeID, b.WorkspaceID, s.workspace, ErrScopeMismatch)
	}
	return nil
}

// remember records a ChangeID in the bounded seen-set, evicting the oldest
// IDs past the bound.
func (s *State) remember(changeID string) {
	s.seen[changeID] = struct{}{}
	s.seenOrder = append(s.seenOrder, changeID)
	if s.seenBound < 0 {
		return
	}
	for len(s.seenOrder)-s.seenHead > s.seenBound {
		delete(s.seen, s.seenOrder[s.seenHead])
		s.seenOrder[s.seenHead] = ""
		s.seenHead++
	}
	// Compact occasionally so the backing array does not grow forever.
	if s.seenHead >= 1024 && s.seenHead*2 >= len(s.seenOrder) {
		s.seenOrder = append([]string(nil), s.seenOrder[s.seenHead:]...)
		s.seenHead = 0
	}
}

// Seen reports whether changeID is in the (bounded) seen-set.
func (s *State) Seen(changeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.seen[changeID]
	return ok
}

// Stats reports how many batches have been applied and absorbed as exact
// duplicates.
func (s *State) Stats() (applied, duplicates uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.applied, s.duplicates
}

// applyOp merges one op at stamp. The caller holds the write lock and has
// validated the batch.
func (s *State) applyOp(op *Op, stamp Stamp) {
	key := recordKey{table: op.Table, id: op.RecordID}
	rec := s.records[key]
	if rec == nil {
		rec = &recordState{fields: make(map[string]fieldState)}
		s.records[key] = rec
	}
	if op.Deleted {
		applyDelete(rec, stamp)
		return
	}
	if !rec.hasAlive || stamp.After(rec.alive) {
		rec.alive, rec.hasAlive = stamp, true
	}
	for name, raw := range op.Fields {
		applyFieldWrite(rec, name, raw, stamp, false)
	}
	for _, name := range op.Removed {
		applyFieldWrite(rec, name, nil, stamp, true)
	}
}

// applyDelete advances the record tombstone and drops fields it covers.
func applyDelete(rec *recordState, stamp Stamp) {
	if rec.hasTomb && !stamp.After(rec.tombstone) {
		return
	}
	rec.tombstone, rec.hasTomb = stamp, true
	for name, f := range rec.fields {
		if !f.stamp.After(stamp) {
			delete(rec.fields, name)
		}
	}
}

// applyFieldWrite merges one field write or removal. Writes at or below the
// tombstone are dropped (the tombstone already covers them); a removal
// stores a per-field tombstone so an older write delivered later still loses.
func applyFieldWrite(rec *recordState, name string, raw json.RawMessage, stamp Stamp, removed bool) {
	if rec.hasTomb && !stamp.After(rec.tombstone) {
		return
	}
	if existing, ok := rec.fields[name]; ok {
		if stamp.Before(existing.stamp) {
			return
		}
		if stamp.Equal(existing.stamp) && !tieBreakWins(raw, removed, existing) {
			return
		}
	}
	next := fieldState{stamp: stamp, deleted: removed}
	if !removed {
		next.value = append(json.RawMessage(nil), raw...)
	}
	rec.fields[name] = next
}

// tieBreakWins resolves the one case stamps cannot: two writes to the same
// field with identical stamps. A correct Clock never issues the same stamp
// twice, but a device restored without observing its snapshot clock could, and
// replicas must still converge. The rule is total and order-independent: a
// value beats a removal (keeping data is recoverable, losing it is not), and
// between two values the lexicographically greater raw JSON wins — replicas
// hold byte-identical copies of the same batch, so they all pick the same
// winner.
func tieBreakWins(raw json.RawMessage, removed bool, existing fieldState) bool {
	switch {
	case removed && existing.deleted:
		return false
	case removed:
		return false // existing value beats an equal-stamp removal
	case existing.deleted:
		return true // incoming value beats an equal-stamp removal
	default:
		return bytes.Compare(raw, existing.value) > 0
	}
}

// Materialize returns the merged JSON document for one record in canonical
// form, and false when the record does not currently exist (never written,
// or deleted with no newer field). An existing but empty document returns
// "{}" and true.
func (s *State) Materialize(table, id string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec := s.records[recordKey{table: table, id: id}]
	if rec == nil {
		return nil, false
	}
	return materializeRecord(rec)
}

// materializeRecord renders one record under a held (read or write) lock.
func materializeRecord(rec *recordState) ([]byte, bool) {
	alive := rec.hasAlive && (!rec.hasTomb || rec.alive.After(rec.tombstone))
	visible := make(map[string]json.RawMessage, len(rec.fields))
	for name, f := range rec.fields {
		if f.deleted {
			continue
		}
		if rec.hasTomb && !f.stamp.After(rec.tombstone) {
			continue
		}
		visible[name] = f.value
	}
	if !alive && len(visible) == 0 {
		return nil, false
	}
	data, err := canonicalObject(visible)
	if err != nil {
		// Field values were validated as JSON before being stored, so this
		// can only fail on a snapshot imported with unchecked values; treat
		// the record as unreadable rather than returning corrupt bytes.
		return nil, false
	}
	return data, true
}

// Exists reports whether the record currently materializes.
func (s *State) Exists(table, id string) bool {
	_, ok := s.Materialize(table, id)
	return ok
}

// Deleted reports whether the record has a tombstone and does not currently
// exist (no field or empty-document write newer than the tombstone).
func (s *State) Deleted(table, id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec := s.records[recordKey{table: table, id: id}]
	if rec == nil || !rec.hasTomb {
		return false
	}
	_, ok := materializeRecord(rec)
	return !ok
}

// Tombstone returns the record's tombstone stamp, if any.
func (s *State) Tombstone(table, id string) (Stamp, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec := s.records[recordKey{table: table, id: id}]
	if rec == nil || !rec.hasTomb {
		return Stamp{}, false
	}
	return rec.tombstone, true
}

// FieldClock returns the winning stamp for one field, whether that winner is
// a removal, and whether the field is known at all.
func (s *State) FieldClock(table, id, field string) (stamp Stamp, deleted, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec := s.records[recordKey{table: table, id: id}]
	if rec == nil {
		return Stamp{}, false, false
	}
	f, ok := rec.fields[field]
	if !ok {
		return Stamp{}, false, false
	}
	return f.stamp, f.deleted, true
}

// RecordIDs returns the sorted IDs of records in table that currently exist.
func (s *State) RecordIDs(table string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for key, rec := range s.records {
		if key.table != table {
			continue
		}
		if _, ok := materializeRecord(rec); ok {
			out = append(out, key.id)
		}
	}
	sort.Strings(out)
	return out
}

// Tables returns the sorted names of tables holding at least one existing
// record.
func (s *State) Tables() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]struct{})
	for key, rec := range s.records {
		if _, ok := materializeRecord(rec); ok {
			seen[key.table] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// MaxStamp returns the greatest stamp stored anywhere in the state (field,
// alive or tombstone), or false when the state is empty. A replica can feed
// the HLC component to Clock.Observe after loading a snapshot so it never
// issues stamps older than the data it holds.
func (s *State) MaxStamp() (Stamp, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best Stamp
	found := false
	consider := func(st Stamp) {
		if !found || st.After(best) {
			best, found = st, true
		}
	}
	for _, rec := range s.records {
		if rec.hasAlive {
			consider(rec.alive)
		}
		if rec.hasTomb {
			consider(rec.tombstone)
		}
		for _, f := range rec.fields {
			consider(f.stamp)
		}
	}
	return best, found
}
