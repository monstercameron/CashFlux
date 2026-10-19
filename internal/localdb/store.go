// SPDX-License-Identifier: MIT

// Package localdb provides a scope-bound encrypted append-only store and the
// replay engine used by the local-first v2 client.
package localdb

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/changes"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

var (
	ErrNotFound = errors.New("localdb: not found")
	ErrConflict = errors.New("localdb: compare-and-swap conflict")
	ErrInvalid  = errors.New("localdb: invalid record")
)

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

// DBName returns the stable IndexedDB name for scope.
func DBName(scope Scope) string {
	if scope.ServerID == "" || scope.ServerID == keyring.LocalServerID {
		return "cfv2:local"
	}
	return "cfv2:" + scope.ServerID + ":" + scope.AccountID
}

// Meta stores device and synchronization cursors for one scope.
type Meta struct {
	DeviceID      string      `json:"deviceId"`
	HLC           changes.HLC `json:"hlc"`
	ServerCursor  uint64      `json:"serverCursor"`
	SchemaVersion int         `json:"schemaVersion"`
	BuildID       string      `json:"buildId"`
	Version       uint64      `json:"version"`
}

// LogEntry is an encrypted change batch. ServerSeq zero means pending.
type LogEntry struct {
	LocalSeq    uint64      `json:"localSeq"`
	ChangeID    string      `json:"changeId"`
	WorkspaceID string      `json:"workspaceId"`
	Sealed      []byte      `json:"sealed"`
	ServerSeq   uint64      `json:"serverSeq"`
	CreatedHLC  changes.HLC `json:"createdHlc"`
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
	AppendRemote(workspaceID, changeID string, sealed []byte, serverSeq uint64, created changes.HLC) (uint64, error)
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
	Raw    []byte
	Reason string
	At     time.Time
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
		if x.ServerSeq == 0 {
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
		if x.WorkspaceID == w && x.LocalSeq <= local && x.ServerSeq != 0 {
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
	m.quarantine = append(m.quarantine, quarantineEntry{clone(b), r, time.Now().UTC()})
	return nil
}
func (m *MemStore) RequestPersistence() (bool, error) {
	if e := m.ready(); e != nil {
		return false, e
	}
	return false, nil
}
func clone(b []byte) []byte         { return append([]byte(nil), b...) }
func copyEntry(x LogEntry) LogEntry { x.Sealed = clone(x.Sealed); return x }
