// SPDX-License-Identifier: MIT

package changes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// FieldClock is one exported merge-clock entry: everything a snapshot needs
// to carry so a replica restored from it keeps merging correctly with
// replicas that replayed the full log.
//
// An entry with a non-empty Field describes that field's winning write (Value
// set, Deleted false) or winning removal (Value empty, Deleted true). An
// entry with an empty Field is a record-level marker: Tombstone true carries
// the delete stamp, Alive true carries the newest non-delete stamp (which is
// what keeps an empty "{}" document distinct from a deleted record). A
// record may therefore export both markers plus one entry per known field.
type FieldClock struct {
	Table     string          `json:"table"`
	RecordID  string          `json:"recordId"`
	Field     string          `json:"field"`
	Value     json.RawMessage `json:"value,omitempty"`
	Stamp     Stamp           `json:"stamp"`
	Deleted   bool            `json:"deleted,omitempty"`
	Tombstone bool            `json:"tombstone,omitempty"`
	Alive     bool            `json:"alive,omitempty"`
}

// ExportFieldClocks returns every merge clock in the state in deterministic
// order (table, record, record markers first, then fields by name), with
// deep-copied canonical values.
func (s *State) ExportFieldClocks() []FieldClock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exportFieldClocksLocked()
}

func (s *State) exportFieldClocksLocked() []FieldClock {
	keys := make([]recordKey, 0, len(s.records))
	for k := range s.records {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].table != keys[j].table {
			return keys[i].table < keys[j].table
		}
		return keys[i].id < keys[j].id
	})
	var out []FieldClock
	for _, key := range keys {
		rec := s.records[key]
		if rec.hasAlive {
			out = append(out, FieldClock{Table: key.table, RecordID: key.id, Stamp: rec.alive, Alive: true})
		}
		if rec.hasTomb {
			out = append(out, FieldClock{Table: key.table, RecordID: key.id, Stamp: rec.tombstone, Tombstone: true})
		}
		names := make([]string, 0, len(rec.fields))
		for name := range rec.fields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			f := rec.fields[name]
			entry := FieldClock{Table: key.table, RecordID: key.id, Field: name, Stamp: f.stamp, Deleted: f.deleted}
			if !f.deleted {
				if canon, err := canonicalJSONValue(f.value); err == nil {
					entry.Value = canon
				} else {
					entry.Value = append(json.RawMessage(nil), f.value...)
				}
			}
			out = append(out, entry)
		}
	}
	return out
}

// ImportFieldClocks merges exported field clocks into the state using the
// same per-field and tombstone rules as Apply, so importing into a non-empty
// state converges instead of overwriting. Every entry is validated before
// anything is written.
func (s *State) ImportFieldClocks(entries []FieldClock) error {
	for i := range entries {
		if err := entries[i].validate(); err != nil {
			return fmt.Errorf("changes: field clock %d: %w", i, err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range entries {
		s.importFieldClockLocked(&entries[i])
	}
	return nil
}

// importFieldClockLocked merges one validated entry. The caller holds the
// write lock.
func (s *State) importFieldClockLocked(e *FieldClock) {
	key := recordKey{table: e.Table, id: e.RecordID}
	rec := s.records[key]
	if rec == nil {
		rec = &recordState{fields: make(map[string]fieldState)}
		s.records[key] = rec
	}
	switch {
	case e.Tombstone:
		applyDelete(rec, e.Stamp)
	case e.Alive:
		if !rec.hasAlive || e.Stamp.After(rec.alive) {
			rec.alive, rec.hasAlive = e.Stamp, true
		}
	default:
		applyFieldWrite(rec, e.Field, e.Value, e.Stamp, e.Deleted)
		// A field entry implies the record was written by a non-delete
		// op at its stamp; keep the alive marker at least that new so a
		// snapshot that (legitimately) omits the record marker still
		// restores the document's existence.
		if !rec.hasAlive || e.Stamp.After(rec.alive) {
			rec.alive, rec.hasAlive = e.Stamp, true
		}
	}
}

// Validate checks one field-clock entry.
func (e FieldClock) Validate() error { return e.validate() }

func (e FieldClock) validate() error {
	if err := checkName("table", e.Table); err != nil {
		return err
	}
	if err := checkName("recordId", e.RecordID); err != nil {
		return err
	}
	if e.Stamp.HLC.WallMillis < 0 {
		return fmt.Errorf("changes: field clock %s/%s has negative wall time: %w", e.Table, e.RecordID, ErrInvalidBatch)
	}
	if err := checkNonEmptyID("stamp.deviceId", e.Stamp.DeviceID); err != nil {
		return err
	}
	if e.Field == "" {
		if e.Tombstone == e.Alive {
			return fmt.Errorf("changes: record marker for %s/%s must set exactly one of tombstone or alive: %w",
				e.Table, e.RecordID, ErrInvalidBatch)
		}
		if len(e.Value) > 0 || e.Deleted {
			return fmt.Errorf("changes: record marker for %s/%s must not carry a value: %w",
				e.Table, e.RecordID, ErrInvalidBatch)
		}
		return nil
	}
	if err := checkName("field name", e.Field); err != nil {
		return err
	}
	if e.Tombstone || e.Alive {
		return fmt.Errorf("changes: field clock %s/%s.%s must not be a record marker: %w",
			e.Table, e.RecordID, e.Field, ErrInvalidBatch)
	}
	if e.Deleted {
		if len(e.Value) > 0 {
			return fmt.Errorf("changes: removed field clock %s/%s.%s must not carry a value: %w",
				e.Table, e.RecordID, e.Field, ErrInvalidBatch)
		}
		return nil
	}
	if len(e.Value) == 0 || !json.Valid(e.Value) {
		return fmt.Errorf("changes: field clock %s/%s.%s value is not valid JSON: %w",
			e.Table, e.RecordID, e.Field, ErrInvalidBatch)
	}
	return nil
}

// Snapshot is the portable form of a State: the field clocks (values plus
// merge stamps) and the retained seen ChangeIDs. Restoring a snapshot and
// then replaying any log suffix converges with a replica that replayed the
// whole log.
type Snapshot struct {
	SchemaVersion int          `json:"schemaVersion"`
	Scope         Scope        `json:"scope"`
	WorkspaceID   string       `json:"workspaceId"`
	FieldClocks   []FieldClock `json:"fieldClocks"`
	SeenChangeIDs []string     `json:"seenChangeIds,omitempty"`
}

// ExportSnapshot captures the state's field clocks and retained seen IDs.
func (s *State) ExportSnapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := Snapshot{
		SchemaVersion: CurrentSchemaVersion,
		Scope:         s.scope,
		WorkspaceID:   s.workspace,
		FieldClocks:   s.exportFieldClocksLocked(),
	}
	if retained := len(s.seenOrder) - s.seenHead; retained > 0 {
		snap.SeenChangeIDs = append([]string(nil), s.seenOrder[s.seenHead:]...)
	}
	return snap
}

// Validate checks a snapshot against this build's supported schema range.
func (snap Snapshot) Validate() error { return snap.validate(MaxSupportedSchemaVersion) }

func (snap Snapshot) validate(supported int) error {
	switch {
	case snap.SchemaVersion > supported:
		return fmt.Errorf("changes: snapshot has schemaVersion %d, newest supported is %d: %w",
			snap.SchemaVersion, supported, ErrNewerSchema)
	case snap.SchemaVersion < MinSchemaVersion:
		return fmt.Errorf("changes: snapshot has schemaVersion %d, oldest supported is %d: %w",
			snap.SchemaVersion, MinSchemaVersion, ErrInvalidBatch)
	}
	for i := range snap.FieldClocks {
		if err := snap.FieldClocks[i].validate(); err != nil {
			return fmt.Errorf("changes: snapshot field clock %d: %w", i, err)
		}
	}
	for _, id := range snap.SeenChangeIDs {
		if !IsUUIDv4(id) {
			return fmt.Errorf("changes: snapshot seen change id %q is not a UUID v4: %w", id, ErrInvalidBatch)
		}
	}
	return nil
}

// ImportSnapshot merges a snapshot into the state (same rules as
// ImportFieldClocks) and adopts its seen ChangeIDs subject to this state's
// seen bound. A scope-bound state requires the snapshot to name the same
// scope (and workspace, when bound); a snapshot naming a different scope is
// rejected with ErrScopeMismatch and nothing is imported.
func (s *State) ImportSnapshot(snap Snapshot) error {
	if err := snap.validate(s.supported); err != nil {
		return err
	}
	if !s.scope.IsZero() && snap.Scope != s.scope {
		return fmt.Errorf("changes: snapshot is for scope %s, state is bound to %s: %w",
			snap.Scope, s.scope, ErrScopeMismatch)
	}
	if s.workspace != "" && snap.WorkspaceID != s.workspace {
		return fmt.Errorf("changes: snapshot is for workspace %q, state is bound to %q: %w",
			snap.WorkspaceID, s.workspace, ErrScopeMismatch)
	}
	if err := s.ImportFieldClocks(snap.FieldClocks); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range snap.SeenChangeIDs {
		if _, dup := s.seen[id]; !dup {
			s.remember(id)
		}
	}
	return nil
}

// MarshalSnapshot encodes a snapshot deterministically (field clocks sorted,
// values canonical, seen IDs in retention order). It validates first.
func MarshalSnapshot(snap Snapshot) ([]byte, error) {
	if err := snap.Validate(); err != nil {
		return nil, err
	}
	out := snap
	out.FieldClocks = normalizeFieldClocks(snap.FieldClocks)
	for i := range out.FieldClocks {
		if len(out.FieldClocks[i].Value) > 0 {
			canon, err := canonicalJSONValue(out.FieldClocks[i].Value)
			if err != nil {
				return nil, fmt.Errorf("changes: snapshot field clock %d: %w: %v", i, ErrInvalidBatch, err)
			}
			out.FieldClocks[i].Value = canon
		}
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("changes: marshal snapshot: %w", err)
	}
	return data, nil
}

// UnmarshalSnapshot decodes a snapshot with strict validation. A snapshot
// from a newer schema is returned (best-effort decoded) together with an
// error wrapping ErrNewerSchema, mirroring Unmarshal for batches.
func UnmarshalSnapshot(data []byte) (Snapshot, error) {
	if len(data) == 0 {
		return Snapshot{}, fmt.Errorf("changes: empty snapshot encoding: %w", ErrInvalidBatch)
	}
	var peek struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &peek); err == nil && peek.SchemaVersion > MaxSupportedSchemaVersion {
		var snap Snapshot
		if derr := json.Unmarshal(data, &snap); derr != nil {
			return Snapshot{}, fmt.Errorf("changes: decode newer-schema snapshot: %w: %v", ErrNewerSchema, derr)
		}
		return snap, fmt.Errorf("changes: snapshot has schemaVersion %d, newest supported is %d: %w",
			snap.SchemaVersion, MaxSupportedSchemaVersion, ErrNewerSchema)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var snap Snapshot
	if err := dec.Decode(&snap); err != nil {
		return Snapshot{}, fmt.Errorf("changes: decode snapshot: %w: %v", ErrInvalidBatch, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Snapshot{}, fmt.Errorf("changes: trailing data after snapshot: %w", ErrInvalidBatch)
	}
	if err := snap.Validate(); err != nil {
		return Snapshot{}, err
	}
	snap.FieldClocks = normalizeFieldClocks(snap.FieldClocks)
	return snap, nil
}

// normalizeFieldClocks deep-copies entries into canonical order.
func normalizeFieldClocks(entries []FieldClock) []FieldClock {
	if entries == nil {
		return nil
	}
	out := make([]FieldClock, len(entries))
	for i, e := range entries {
		out[i] = e
		if e.Value != nil {
			out[i].Value = append(json.RawMessage(nil), e.Value...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return fieldClockLess(out[i], out[j]) })
	return out
}

// fieldClockLess is the canonical field-clock ordering: table, record,
// record markers before fields, then field name, then stamp.
func fieldClockLess(a, b FieldClock) bool {
	if a.Table != b.Table {
		return a.Table < b.Table
	}
	if a.RecordID != b.RecordID {
		return a.RecordID < b.RecordID
	}
	if rank := markerRank(a) - markerRank(b); rank != 0 {
		return rank < 0
	}
	if a.Field != b.Field {
		return a.Field < b.Field
	}
	return a.Stamp.Before(b.Stamp)
}

func markerRank(e FieldClock) int {
	switch {
	case e.Alive:
		return 0
	case e.Tombstone:
		return 1
	default:
		return 2
	}
}
