// SPDX-License-Identifier: MIT

// Package localdb provides a scope-bound encrypted append-only store and the
// replay engine used by the local-first v2 client.
package localdb

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/changes"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

var (
	ErrNotFound      = errors.New("localdb: not found")
	ErrConflict      = errors.New("localdb: compare-and-swap conflict")
	ErrInvalid       = errors.New("localdb: invalid record")
	ErrScopeMismatch = errors.New("localdb: scope mismatch")
)

// ScopeMismatchError reports that a database's stored scope differs from the
// scope requested by its opener.
type ScopeMismatchError struct {
	Expected Scope
	Actual   Scope
}

func (e *ScopeMismatchError) Error() string {
	return fmt.Sprintf("localdb: database scope mismatch: stored %v, requested %v", e.Actual, e.Expected)
}

// Is allows callers to classify scope mismatch failures with errors.Is.
func (e *ScopeMismatchError) Is(target error) bool { return target == ErrScopeMismatch }

// QuotaError identifies browser storage exhaustion. It is returned only for
// the IndexedDB QuotaExceededError failure, never for other storage errors,
// so the UI can safely say "storage is full" when it sees one.
type QuotaError struct {
	// Name is the DOMException name, normally "QuotaExceededError".
	Name string
	Err  error
}

func (e *QuotaError) Error() string { return fmt.Sprintf("localdb: storage quota exceeded: %v", e.Err) }
func (e *QuotaError) Unwrap() error { return e.Err }

// Sentinel errors classifying non-quota browser storage failures. Test for
// them with errors.Is against an error returned by a Store; the underlying
// *StorageError also carries the original DOMException name.
var (
	// ErrStorageAborted means the browser aborted a storage transaction
	// (DOMException AbortError). Nothing from that transaction was stored.
	ErrStorageAborted = errors.New("localdb: storage transaction aborted")
	// ErrStorageSecurity means the browser refused storage for security or
	// privacy reasons (DOMException SecurityError), for example a blocked
	// third-party context.
	ErrStorageSecurity = errors.New("localdb: storage blocked by browser security policy")
	// ErrStorageUnsupported means browser storage is unavailable or lacks a
	// required feature (DOMException NotSupportedError, or no IndexedDB).
	ErrStorageUnsupported = errors.New("localdb: storage not supported in this browser")
)

// StorageError is a browser storage failure that is not quota exhaustion.
// Name carries the original DOMException name (for example "AbortError") so
// callers and logs keep the precise cause instead of a generic message.
type StorageError struct {
	// Name is the DOMException name, or empty when the browser gave none.
	Name string
	Err  error
}

func (e *StorageError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("localdb: storage error %s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("localdb: storage error: %v", e.Err)
}
func (e *StorageError) Unwrap() error { return e.Err }

// Is matches the sentinel for the DOMException name this error carries.
func (e *StorageError) Is(target error) bool {
	switch target {
	case ErrStorageAborted:
		return e.Name == "AbortError"
	case ErrStorageSecurity:
		return e.Name == "SecurityError"
	case ErrStorageUnsupported:
		return e.Name == "NotSupportedError"
	}
	return false
}

// classifyStorageError maps a browser DOMException name and message to the
// typed error the rest of the app handles. QuotaExceededError — and only
// that name — becomes *QuotaError; everything else becomes *StorageError
// carrying the name. It is pure so the mapping is tested on native Go.
func classifyStorageError(name, message string) error {
	cause := message
	if cause == "" {
		cause = name
	}
	if cause == "" {
		cause = "unknown IndexedDB failure"
	}
	base := errors.New(cause)
	if name == "QuotaExceededError" {
		return &QuotaError{Name: name, Err: base}
	}
	return &StorageError{Name: name, Err: base}
}

// storageErrorName returns the DOMException name carried by err, or "".
func storageErrorName(err error) string {
	var quota *QuotaError
	if errors.As(err, &quota) {
		return quota.Name
	}
	var storage *StorageError
	if errors.As(err, &storage) {
		return storage.Name
	}
	return ""
}

// Scope is the account boundary shared by keyring and changes.
type Scope = keyring.Scope

// DBName returns the stable IndexedDB name for scope. Local scopes use
// "cfv2:local". Other scopes use "cfv2:" followed by the lowercase hex
// SHA-256 of the uint64-length-prefixed ServerID and AccountID byte strings.
// Length-prefixing prevents delimiter ambiguity and hashing keeps names
// independent of the IDs' contents and length.
func DBName(scope Scope) string {
	if scope.ServerID == "" || scope.ServerID == keyring.LocalServerID {
		return "cfv2:local"
	}
	h := sha256.New()
	for _, field := range []string{scope.ServerID, scope.AccountID} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(field))
	}
	return "cfv2:" + hex.EncodeToString(h.Sum(nil))
}

// Meta stores device and synchronization cursors for one scope.
type Meta struct {
	Scope         Scope       `json:"scope"`
	DeviceID      string      `json:"deviceId"`
	HLC           changes.HLC `json:"hlc"`
	ServerCursor  uint64      `json:"serverCursor"`
	SchemaVersion int         `json:"schemaVersion"`
	BuildID       string      `json:"buildId"`
	Version       uint64      `json:"version"`
}

// LogEntry is an encrypted change batch. ServerSeq zero means pending.
type LogEntry struct {
	LocalSeq     uint64      `json:"localSeq"`
	ChangeID     string      `json:"changeId"`
	WorkspaceID  string      `json:"workspaceId"`
	Sealed       []byte      `json:"sealed"`
	ServerSeq    uint64      `json:"serverSeq"`
	CreatedHLC   changes.HLC `json:"createdHlc"`
	Rejected     bool        `json:"rejected,omitempty"`
	RejectReason string      `json:"rejectReason,omitempty"`
}

// RejectedRecord identifies one rejected log position without relying on
// ciphertext equality, which may also occur on unrelated remote payloads.
type RejectedRecord struct {
	Scope       Scope  `json:"scope"`
	WorkspaceID string `json:"workspaceId"`
	ChangeID    string `json:"changeId"`
	LocalSeq    uint64 `json:"localSeq"`
	ServerSeq   uint64 `json:"serverSeq"`
	SHA256      string `json:"sha256"`
	Reason      string `json:"reason"`
}

// Snapshot is a sealed workspace state and the log positions it covers.
type Snapshot struct {
	WorkspaceID     string `json:"workspaceId"`
	CoversLocalSeq  uint64 `json:"coversLocalSeq"`
	CoversServerSeq uint64 `json:"coversServerSeq"`
	Sealed          []byte `json:"sealed"`
}

// Store persists encrypted data in exactly one scope.
type Store interface {
	Open(scope Scope) error
	Meta() (Meta, error)
	PutMeta(meta Meta) error
	Keyring() ([]byte, error)
	PutKeyring(blob []byte, expectedVersion uint64) error
	Append(workspaceID, changeID string, sealed []byte, created changes.HLC) (uint64, error)
	Pending(limit int) ([]LogEntry, error)
	MarkAcked(localSeq, serverSeq uint64) error
	RejectLogEntry(localSeq uint64, reason string) (RejectedRecord, error)
	RejectedLogEntries() ([]RejectedRecord, error)
	AppendRemote(workspaceID, changeID string, sealed []byte, serverSeq uint64, created changes.HLC) (uint64, error)
	IsQuarantined(raw []byte) (bool, error)
	Since(localSeq uint64) ([]LogEntry, error)
	LatestSnapshot(workspaceID string) (Snapshot, error)
	Compact(workspaceID string, sealed []byte, coversLocalSeq, coversServerSeq uint64) error
	PutDraft(key string, sealed []byte) error
	GetDraft(key string) ([]byte, error)
	DeleteDraft(key string) error
	Quarantine(raw []byte, reason string) error
	RequestPersistence() (bool, error)
}

type quarantineEntry struct {
	Raw      []byte
	Reason   string
	At       time.Time
	Rejected *RejectedRecord `json:"rejected,omitempty"`
}

// MemStore is a concurrency-safe store for native use and conformance tests.
type MemStore struct {
	mu             sync.Mutex
	scope          Scope
	opened         bool
	meta           Meta
	keyring        []byte
	keyringVersion uint64
	log            []LogEntry
	nextSeq        uint64
	snapshots      map[string]Snapshot
	drafts         map[string][]byte
	quarantine     []quarantineEntry
}

// NewMemStore creates an empty in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{snapshots: map[string]Snapshot{}, drafts: map[string][]byte{}}
}
func (m *MemStore) Open(s Scope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.opened && m.scope != s {
		return fmt.Errorf("open scope: %w", ErrConflict)
	}
	m.scope = s
	if m.meta.Scope != (Scope{}) && m.meta.Scope != s {
		return &ScopeMismatchError{Expected: s, Actual: m.meta.Scope}
	}
	m.meta.Scope = s
	m.opened = true
	return nil
}
func (m *MemStore) ready() error {
	if !m.opened {
		return fmt.Errorf("store is not open: %w", ErrInvalid)
	}
	return nil
}
func (m *MemStore) Meta() (Meta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return Meta{}, e
	}
	return m.meta, nil
}
func (m *MemStore) PutMeta(v Meta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return e
	}
	v.Scope = m.scope
	v.Version = m.meta.Version + 1
	m.meta = v
	return nil
}
func (m *MemStore) Keyring() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return nil, e
	}
	if m.keyring == nil {
		return nil, ErrNotFound
	}
	return clone(m.keyring), nil
}
func (m *MemStore) PutKeyring(b []byte, expected uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return e
	}
	if m.keyringVersion != expected {
		return ErrConflict
	}
	m.keyring = clone(b)
	m.keyringVersion++
	return nil
}
func (m *MemStore) Append(w, id string, b []byte, h changes.HLC) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return 0, e
	}
	if w == "" || id == "" || len(b) == 0 {
		return 0, ErrInvalid
	}
	for _, x := range m.log {
		if x.ChangeID == id {
			return 0, ErrConflict
		}
	}
	m.nextSeq++
	m.log = append(m.log, LogEntry{LocalSeq: m.nextSeq, WorkspaceID: w, ChangeID: id, Sealed: clone(b), CreatedHLC: h})
	return m.nextSeq, nil
}
func (m *MemStore) Pending(limit int) ([]LogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return nil, e
	}
	if limit <= 0 {
		return []LogEntry{}, nil
	}
	out := []LogEntry{}
	for _, x := range m.log {
		if x.ServerSeq == 0 && !x.Rejected {
			out = append(out, copyEntry(x))
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}
func (m *MemStore) MarkAcked(seq, server uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return e
	}
	if server == 0 {
		return ErrInvalid
	}
	for i := range m.log {
		if m.log[i].LocalSeq == seq {
			if m.log[i].ServerSeq != 0 && m.log[i].ServerSeq != server {
				return ErrConflict
			}
			m.log[i].ServerSeq = server
			return nil
		}
	}
	return ErrNotFound
}

// RejectLogEntry marks exactly one log position rejected and records its
// ciphertext digest and scope metadata in quarantine atomically.
func (m *MemStore) RejectLogEntry(seq uint64, reason string) (RejectedRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ready(); err != nil {
		return RejectedRecord{}, err
	}
	if reason == "" {
		return RejectedRecord{}, ErrInvalid
	}
	for i := range m.log {
		entry := &m.log[i]
		if entry.LocalSeq != seq {
			continue
		}
		if entry.Rejected {
			return rejectionFor(m.scope, *entry, entry.RejectReason), nil
		}
		record := rejectionFor(m.scope, *entry, reason)
		entry.Rejected = true
		entry.RejectReason = reason
		m.quarantine = append(m.quarantine, quarantineEntry{Raw: clone(entry.Sealed), Reason: reason, At: time.Now().UTC(), Rejected: &record})
		return record, nil
	}
	return RejectedRecord{}, ErrNotFound
}

// RejectedLogEntries returns a stable list ordered by local sequence.
func (m *MemStore) RejectedLogEntries() ([]RejectedRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ready(); err != nil {
		return nil, err
	}
	var out []RejectedRecord
	for _, entry := range m.log {
		if entry.Rejected {
			out = append(out, rejectionFor(m.scope, entry, entry.RejectReason))
		}
	}
	return out, nil
}

func rejectionFor(scope Scope, entry LogEntry, reason string) RejectedRecord {
	digest := sha256.Sum256(entry.Sealed)
	return RejectedRecord{Scope: scope, WorkspaceID: entry.WorkspaceID, ChangeID: entry.ChangeID, LocalSeq: entry.LocalSeq, ServerSeq: entry.ServerSeq, SHA256: hex.EncodeToString(digest[:]), Reason: reason}
}
func (m *MemStore) AppendRemote(w, id string, b []byte, server uint64, h changes.HLC) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return 0, e
	}
	if w == "" || id == "" || len(b) == 0 || server == 0 {
		return 0, ErrInvalid
	}
	for _, x := range m.log {
		if x.ChangeID == id {
			if x.WorkspaceID != w || x.ServerSeq != server || !bytes.Equal(x.Sealed, b) {
				return 0, ErrConflict
			}
			return x.LocalSeq, nil
		}
	}
	m.nextSeq++
	m.log = append(m.log, LogEntry{LocalSeq: m.nextSeq, WorkspaceID: w, ChangeID: id, Sealed: clone(b), ServerSeq: server, CreatedHLC: h})
	return m.nextSeq, nil
}
func (m *MemStore) Since(seq uint64) ([]LogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return nil, e
	}
	out := []LogEntry{}
	for _, x := range m.log {
		if x.LocalSeq > seq {
			out = append(out, copyEntry(x))
		}
	}
	return out, nil
}
func (m *MemStore) LatestSnapshot(w string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return Snapshot{}, e
	}
	s, ok := m.snapshots[w]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	s.Sealed = clone(s.Sealed)
	return s, nil
}
func (m *MemStore) Compact(w string, b []byte, local, server uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return e
	}
	if w == "" || len(b) == 0 {
		return ErrInvalid
	}
	s := Snapshot{WorkspaceID: w, CoversLocalSeq: local, CoversServerSeq: server, Sealed: clone(b)}
	m.snapshots[w] = s
	kept := m.log[:0]
	for _, x := range m.log {
		if x.WorkspaceID == w && x.LocalSeq <= local && x.ServerSeq != 0 && !x.Rejected {
			continue
		}
		kept = append(kept, x)
	}
	m.log = kept
	return nil
}
func (m *MemStore) PutDraft(k string, b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return e
	}
	if k == "" {
		return ErrInvalid
	}
	m.drafts[k] = clone(b)
	return nil
}
func (m *MemStore) GetDraft(k string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return nil, e
	}
	b, ok := m.drafts[k]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(b), nil
}
func (m *MemStore) DeleteDraft(k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return e
	}
	delete(m.drafts, k)
	return nil
}
func (m *MemStore) Quarantine(b []byte, r string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return e
	}
	if r == "" {
		return ErrInvalid
	}
	m.quarantine = append(m.quarantine, quarantineEntry{Raw: clone(b), Reason: r, At: time.Now().UTC()})
	return nil
}

// IsQuarantined reports whether raw bytes were already recorded in quarantine.
func (m *MemStore) IsQuarantined(raw []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.ready(); e != nil {
		return false, e
	}
	for _, entry := range m.quarantine {
		if bytes.Equal(entry.Raw, raw) {
			return true, nil
		}
	}
	return false, nil
}
func (m *MemStore) RequestPersistence() (bool, error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	return false, nil
}
func clone(b []byte) []byte         { return append([]byte(nil), b...) }
func copyEntry(x LogEntry) LogEntry { x.Sealed = clone(x.Sealed); return x }
