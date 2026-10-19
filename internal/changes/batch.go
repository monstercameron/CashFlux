// SPDX-License-Identifier: MIT

// Package changes defines the encrypted-local-first v2 change-batch model:
// every user action is captured as one Batch, devices and tabs converge by
// applying batches to a State, and all merge semantics live here in pure Go
// (the server only stores ciphertext and never merges).
//
// Merge rules in one paragraph: each batch carries one Stamp — a hybrid
// logical clock value plus the issuing DeviceID. A field is overwritten only
// by a strictly greater Stamp. A record-level delete records a tombstone
// Stamp; only field writes with a strictly greater Stamp revive the record,
// and then only with those newer fields. Because every decision is a
// per-field comparison against a stored Stamp, applying any set of batches is
// idempotent, commutative and associative: replicas that have seen the same
// batches converge to byte-identical documents regardless of arrival order.
package changes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// CurrentSchemaVersion is the batch schema this build writes.
const CurrentSchemaVersion = 1

// MinSchemaVersion is the oldest batch schema this build can read.
const MinSchemaVersion = 1

// MaxSupportedSchemaVersion is the newest batch schema this build can read.
// Batches with a higher version are rejected with ErrNewerSchema so a newer
// client's data is never merged (and thereby lost) by an older client.
const MaxSupportedSchemaVersion = CurrentSchemaVersion

// Limits rejected by validation and the codec. They exist so a corrupt or
// hostile payload fails fast instead of allocating unbounded memory.
const (
	// MaxBatchBytes caps the encoded form of one batch (1 MiB).
	MaxBatchBytes = 1 << 20
	// MaxOpsPerBatch caps the number of record operations in one batch.
	MaxOpsPerBatch = 10_000
	// MaxFieldsPerOp caps the fields (writes plus removals) in one op.
	MaxFieldsPerOp = 10_000
	// MaxJSONDepth caps nesting in the batch and its JSON field values.
	MaxJSONDepth = 64
	// MaxNameBytes caps table, record and field names.
	MaxNameBytes = 512
	// MaxIDBytes caps scope, workspace and device identifiers.
	MaxIDBytes = 256
)

// Sentinel errors. All validation failures wrap ErrInvalidBatch; schema and
// scope failures have their own sentinels so callers can branch with
// errors.Is.
var (
	// ErrInvalidBatch marks every structural validation failure.
	ErrInvalidBatch = errors.New("changes: invalid batch")
	// ErrNewerSchema marks a batch (or snapshot) written by a newer schema
	// than this build supports. Nothing from it is applied.
	ErrNewerSchema = errors.New("changes: schema version is newer than supported")
	// ErrScopeMismatch marks a batch whose scope or workspace differs from
	// the scope a State is bound to. Nothing from it is applied.
	ErrScopeMismatch = errors.New("changes: batch scope does not match state scope")
	// ErrClockSkew is reported by Clock.Observe when a remote wall time
	// exceeded the configured maximum skew and was clamped.
	ErrClockSkew = errors.New("changes: remote clock is too far ahead")
	// ErrInvalidDocument marks a document that is not a JSON object in a diff.
	ErrInvalidDocument = errors.New("changes: document is not a JSON object")
)

// Scope names the security boundary every byte belongs to: one account on one
// server. A local-only scope uses LocalScope. Batches from different scopes
// must never be merged into one State.
type Scope struct {
	ServerID  string `json:"serverId"`
	AccountID string `json:"accountId"`
}

// LocalScope is the scope of an offline, never-synced dataset.
var LocalScope = Scope{ServerID: "local", AccountID: "local"}

// IsZero reports whether the scope carries no identifiers. A zero Scope is
// invalid in a batch; for a State it means "not bound to a scope".
func (s Scope) IsZero() bool { return s.ServerID == "" && s.AccountID == "" }

// String renders the scope as "<serverId>/<accountId>".
func (s Scope) String() string { return s.ServerID + "/" + s.AccountID }

// Op is one record-level operation inside a Batch.
//
// Fields holds the top-level JSON fields being written, each value a raw JSON
// document. Nested objects are replaced whole: a write to "address" replaces
// the entire address object rather than merging into it (see DiffDocuments).
// Removed lists top-level field names being deleted from the record; each
// removal is merged with its own per-field stamp exactly like a write, so an
// older write arriving later cannot resurrect a removed field. Deleted marks
// removal of the whole record (a tombstone); it excludes Fields and Removed.
//
// A non-deleted Op with no Fields and no Removed is the representation of an
// empty JSON object document ("{}").
type Op struct {
	Table    string                     `json:"table"`
	RecordID string                     `json:"recordId"`
	Fields   map[string]json.RawMessage `json:"fields,omitempty"`
	Removed  []string                   `json:"removed,omitempty"`
	Deleted  bool                       `json:"deleted,omitempty"`
}

// Batch is the sealed result of one user action: a set of record operations
// issued by one device at one HLC. ChangeID (a random UUID v4) is the
// idempotency key; it must never be reused for different content.
type Batch struct {
	ChangeID      string `json:"changeId"`
	Scope         Scope  `json:"scope"`
	WorkspaceID   string `json:"workspaceId"`
	DeviceID      string `json:"deviceId"`
	HLC           HLC    `json:"hlc"`
	SchemaVersion int    `json:"schemaVersion"`
	Ops           []Op   `json:"ops"`
}

// Stamp returns the ordering key shared by every op in the batch.
func (b Batch) Stamp() Stamp { return Stamp{HLC: b.HLC, DeviceID: b.DeviceID} }

// NewChangeID returns a random UUID v4 string using crypto/rand.
func NewChangeID() (string, error) { return newUUIDv4(rand.Reader) }

// newUUIDv4 renders 16 random bytes from src as a canonical UUID v4.
func newUUIDv4(src io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(src, b[:]); err != nil {
		return "", fmt.Errorf("changes: read random source for change id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:]), nil
}

// IsUUIDv4 reports whether s is a canonical UUID v4 (lowercase or uppercase
// hex, version nibble 4, variant 8/9/a/b).
func IsUUIDv4(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	for i := 0; i < 36; i++ {
		switch i {
		case 8, 13, 18, 23:
			continue
		case 14:
			if s[i] != '4' {
				return false
			}
		case 19:
			switch s[i] {
			case '8', '9', 'a', 'b', 'A', 'B':
			default:
				return false
			}
		default:
			if !isHex(s[i]) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// NewBatch builds a validated Batch with a fresh random ChangeID at the
// current schema version. Ops are deep-copied and normalized (sorted), so
// later mutation of the caller's slices cannot affect the batch.
func NewBatch(scope Scope, workspaceID, deviceID string, hlc HLC, ops []Op) (Batch, error) {
	id, err := NewChangeID()
	if err != nil {
		return Batch{}, err
	}
	b := Batch{
		ChangeID:      id,
		Scope:         scope,
		WorkspaceID:   workspaceID,
		DeviceID:      deviceID,
		HLC:           hlc,
		SchemaVersion: CurrentSchemaVersion,
		Ops:           normalizeOps(ops),
	}
	if err := b.Validate(); err != nil {
		return Batch{}, err
	}
	return b, nil
}

// Clone returns a deep copy of the batch.
func (b Batch) Clone() Batch {
	out := b
	out.Ops = cloneOps(b.Ops)
	return out
}

// cloneOps deep-copies ops without sorting them.
func cloneOps(ops []Op) []Op {
	if ops == nil {
		return nil
	}
	out := make([]Op, len(ops))
	for i, op := range ops {
		out[i] = cloneOp(op)
	}
	return out
}

// cloneOp deep-copies one op.
func cloneOp(op Op) Op {
	out := op
	if op.Fields != nil {
		out.Fields = make(map[string]json.RawMessage, len(op.Fields))
		for k, v := range op.Fields {
			out.Fields[k] = append(json.RawMessage(nil), v...)
		}
	}
	if op.Removed != nil {
		out.Removed = append([]string(nil), op.Removed...)
	}
	return out
}

// normalizeOps deep-copies ops into canonical order: ops sorted by
// (table, record), removed-field lists sorted. Map iteration order never
// leaks into a normalized batch.
func normalizeOps(ops []Op) []Op {
	out := cloneOps(ops)
	for i := range out {
		sort.Strings(out[i].Removed)
	}
	sort.Slice(out, func(i, j int) bool { return opLess(out[i], out[j]) })
	return out
}

// opLess is the canonical op ordering.
func opLess(a, b Op) bool {
	if a.Table != b.Table {
		return a.Table < b.Table
	}
	if a.RecordID != b.RecordID {
		return a.RecordID < b.RecordID
	}
	return !a.Deleted && b.Deleted
}
