// SPDX-License-Identifier: MIT
//go:build js && wasm

package localdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/changes"
)

// IndexedDB layout for one scope database (see DBName). Every record kind
// lives in its own object store as its own record; nothing is checkpointed
// as a whole-database blob, so two tabs can never overwrite each other by
// rewriting shared state:
//
//	meta       "meta" -> Meta JSON; "counters" -> sequence counters JSON
//	keyring    "keyring" -> {version, blob} JSON (compare-and-swap by version)
//	log        localSeq (number) -> {changeId, workspaceId, serverSeq, data}
//	           where data is the LogEntry JSON; unique index on changeId
//	snapshot   workspaceID -> Snapshot JSON
//	drafts     draft key -> sealed bytes (JSON/base64)
//	quarantine sequence (number) -> quarantine entry JSON
//
// A write returns only after its transaction's complete event, with
// durability "strict". There is no in-memory mirror to publish into: reads
// always come from IndexedDB, so a write whose transaction aborts is never
// visible anywhere, and a committed write is visible to every tab. Sequence
// allocation happens inside the same readwrite transaction as the log
// insert; IndexedDB serializes overlapping readwrite transactions, so two
// tabs appending at once each get their own localSeq and both entries
// survive.
const (
	idbStoreMeta       = "meta"
	idbStoreKeyring    = "keyring"
	idbStoreLog        = "log"
	idbStoreSnapshot   = "snapshot"
	idbStoreDrafts     = "drafts"
	idbStoreQuarantine = "quarantine"

	idbMetaKey       = "meta"
	idbCountersKey   = "counters"
	idbKeyringKey    = "keyring"
	idbLegacyState   = "state" // pre-SV3a.1 whole-state checkpoint, migrated on Open
	idbLogChangeIdx  = "changeId"
	idbSchemaVersion = 1
)

var idbAllStores = []string{
	idbStoreMeta, idbStoreKeyring, idbStoreLog,
	idbStoreSnapshot, idbStoreDrafts, idbStoreQuarantine,
}

// counters holds the durable sequence generators. It lives in the meta
// store and is only ever read and written inside the same transaction as
// the record that consumes a sequence, so an aborted write rolls the
// counter back too and consumes nothing.
type counters struct {
	NextSeq       uint64 `json:"nextSeq"`
	QuarantineSeq uint64 `json:"quarantineSeq"`
}

// keyringRecord is the stored form of the opaque keyring blob.
type keyringRecord struct {
	Version uint64 `json:"version"`
	Blob    []byte `json:"blob"`
}

// diskState is the pre-SV3a.1 checkpoint format (one meta/state record
// holding the whole database). It is kept only so Open can migrate such a
// database into per-store records without losing anything.
type diskState struct {
	Scope          Scope               `json:"scope"`
	Meta           Meta                `json:"meta"`
	Keyring        []byte              `json:"keyring,omitempty"`
	KeyringVersion uint64              `json:"keyringVersion"`
	Log            []LogEntry          `json:"log"`
	NextSeq        uint64              `json:"nextSeq"`
	Snapshots      map[string]Snapshot `json:"snapshots"`
	Drafts         map[string][]byte   `json:"drafts"`
	Quarantine     []quarantineEntry   `json:"quarantine"`
}

// IDBStore is the browser Store backed by one per-scope IndexedDB database.
// Methods are serialized per instance; cross-tab safety comes from
// IndexedDB transaction semantics, not from this mutex.
type IDBStore struct {
	mu       sync.Mutex
	db       js.Value
	dbName   string
	scope    Scope
	scopeSet bool // a scope was attempted; the instance is bound to it
	opened   bool
}

// NewIDBStore creates an IndexedDB-backed store. Call Open before use.
func NewIDBStore() *IDBStore { return &IDBStore{} }

// domNameMessage extracts the DOMException name and message from a JS error
// value, tolerating nulls and non-error values.
func domNameMessage(v js.Value) (name, message string) {
	if !v.Truthy() {
		return "", ""
	}
	if n := v.Get("name"); n.Type() == js.TypeString {
		name = n.String()
	}
	if m := v.Get("message"); m.Type() == js.TypeString {
		message = m.String()
	}
	if name == "" && message == "" {
		message = v.String()
	}
	return name, message
}

// jsFailure maps a JS error value to the typed localdb error for its
// DOMException name.
func jsFailure(v js.Value) error {
	name, message := domNameMessage(v)
	return classifyStorageError(name, message)
}

// panicFailure converts a recovered syscall/js panic (a thrown JS
// DOMException) into the typed localdb error for its name.
func panicFailure(recovered any) error {
	switch v := recovered.(type) {
	case js.Error:
		return jsFailure(v.Value)
	case js.Value:
		return jsFailure(v)
	default:
		return fmt.Errorf("IndexedDB call failed: %v", recovered)
	}
}

// callValue runs fn and converts a thrown JS exception into an error.
func callValue(fn func() js.Value) (value js.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = panicFailure(r)
		}
	}()
	return fn(), nil
}

// awaitRequest waits for one IndexedDB request and returns its result. The
// handlers are installed synchronously, before the request can settle.
func awaitRequest(req js.Value) (js.Value, error) {
	ch := make(chan struct{})
	var result js.Value
	var failure error
	var once sync.Once
	ok := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		once.Do(func() { result = req.Get("result"); close(ch) })
		return nil
	})
	bad := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		once.Do(func() { failure = jsFailure(req.Get("error")); close(ch) })
		return nil
	})
	req.Set("onsuccess", ok)
	req.Set("onerror", bad)
	<-ch
	ok.Release()
	bad.Release()
	return result, failure
}

// awaitPromise waits for a JS promise (used for navigator.storage.persist).
func awaitPromise(p js.Value) (js.Value, error) {
	ch := make(chan struct{})
	var value js.Value
	var failure error
	var once sync.Once
	ok := js.FuncOf(func(_ js.Value, a []js.Value) any {
		once.Do(func() { value = a[0]; close(ch) })
		return nil
	})
	bad := js.FuncOf(func(_ js.Value, a []js.Value) any {
		once.Do(func() { failure = jsFailure(a[0]); close(ch) })
		return nil
	})
	p.Call("then", ok).Call("catch", bad)
	<-ch
	ok.Release()
	bad.Release()
	return value, failure
}

// txWaiter captures a transaction's outcome. It is installed before any
// request is placed, so an abort that follows a failed request cannot be
// missed, and it detaches its handlers on first settlement so the paired
// error/abort events never call a released function.
type txWaiter struct {
	ch    chan struct{}
	err   error
	once  sync.Once
	funcs []js.Func
}

func watchTx(tx js.Value) *txWaiter {
	w := &txWaiter{ch: make(chan struct{})}
	settle := func(err error) {
		w.once.Do(func() {
			w.err = err
			tx.Set("oncomplete", js.Null())
			tx.Set("onerror", js.Null())
			tx.Set("onabort", js.Null())
			close(w.ch)
		})
	}
	complete := js.FuncOf(func(_ js.Value, _ []js.Value) any { settle(nil); return nil })
	failed := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		err := jsFailure(tx.Get("error"))
		settle(err)
		return nil
	})
	tx.Set("oncomplete", complete)
	tx.Set("onerror", failed)
	tx.Set("onabort", failed)
	w.funcs = []js.Func{complete, failed}
	return w
}

// wait blocks until the transaction commits or aborts and returns the
// failure, if any. A nil error means the complete event fired: the write
// is durable and only then may the caller treat it as published.
func (w *txWaiter) wait() error {
	<-w.ch
	for _, f := range w.funcs {
		f.Release()
	}
	return w.err
}

// newTx opens a transaction over stores. Readwrite transactions request
// strict durability so a successful write is not acknowledged with weaker
// durability guarantees.
func (s *IDBStore) newTx(stores []string, mode string) (js.Value, *txWaiter, error) {
	names := js.Global().Get("Array").New(len(stores))
	for i, n := range stores {
		names.SetIndex(i, n)
	}
	tx, err := callValue(func() js.Value {
		if mode == "readwrite" {
			opts := js.Global().Get("Object").New()
			opts.Set("durability", "strict")
			return s.db.Call("transaction", names, mode, opts)
		}
		return s.db.Call("transaction", names, mode)
	})
	if err != nil {
		return js.Undefined(), nil, fmt.Errorf("open IndexedDB transaction: %w", err)
	}
	return tx, watchTx(tx), nil
}

func txStore(tx js.Value, name string) (js.Value, error) {
	store, err := callValue(func() js.Value { return tx.Call("objectStore", name) })
	if err != nil {
		return js.Undefined(), fmt.Errorf("open object store %q: %w", name, err)
	}
	return store, nil
}

// abortTx aborts tx and waits for it to settle, discarding the abort error
// itself: the caller has a more meaningful semantic error to return.
func abortTx(tx js.Value, w *txWaiter) {
	_, _ = callValue(func() js.Value { tx.Call("abort"); return js.Undefined() })
	_ = w.wait()
}

// Open opens (creating if needed) the IndexedDB database for scope and
// migrates a legacy whole-state checkpoint, if one is present, into
// per-store records. Opening the same scope twice is a no-op; opening a
// different scope on the same instance is a conflict, matching MemStore.
func (s *IDBStore) Open(scope Scope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scopeSet && s.scope != scope {
		return fmt.Errorf("open scope: %w", ErrConflict)
	}
	if s.opened {
		return nil
	}
	indexed := js.Global().Get("indexedDB")
	if indexed.IsUndefined() || indexed.IsNull() {
		return &StorageError{Name: "NotSupportedError", Err: fmt.Errorf("IndexedDB is not available in this browser")}
	}
	s.dbName = DBName(scope)
	s.scope = scope
	s.scopeSet = true
	req, err := callValue(func() js.Value { return indexed.Call("open", s.dbName, idbSchemaVersion) })
	if err != nil {
		return fmt.Errorf("open IndexedDB %q: %w", s.dbName, err)
	}
	ch := make(chan struct{})
	var failure error
	var once sync.Once
	upgrade := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		db := req.Get("result")
		names := db.Get("objectStoreNames")
		for _, name := range idbAllStores {
			if names.Call("contains", name).Bool() {
				continue
			}
			store := db.Call("createObjectStore", name)
			if name == idbStoreLog {
				opts := js.Global().Get("Object").New()
				opts.Set("unique", true)
				store.Call("createIndex", idbLogChangeIdx, "changeId", opts)
			}
		}
		return nil
	})
	success := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		once.Do(func() { s.db = req.Get("result"); close(ch) })
		return nil
	})
	failed := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		once.Do(func() { failure = jsFailure(req.Get("error")); close(ch) })
		return nil
	})
	req.Set("onupgradeneeded", upgrade)
	req.Set("onsuccess", success)
	req.Set("onerror", failed)
	<-ch
	upgrade.Release()
	success.Release()
	failed.Release()
	if failure != nil {
		return fmt.Errorf("open IndexedDB %q: %w", s.dbName, failure)
	}
	if err := s.migrateLegacyLocked(); err != nil {
		// Leave opened=false so a later Open retries the migration; the
		// legacy record is untouched because migration is one transaction.
		return err
	}
	if err := s.ensureScopeLocked(); err != nil {
		return err
	}
	s.opened = true
	return nil
}

// ensureScopeLocked creates the scope binding in the meta record on first
// open and refuses a database whose recorded scope differs. It is a single
// transaction so concurrent tabs cannot observe an unbound database.
func (s *IDBStore) ensureScopeLocked() error {
	tx, w, err := s.newTx([]string{idbStoreMeta}, "readwrite")
	if err != nil {
		return err
	}
	value, found, err := getInTx(tx, idbStoreMeta, idbMetaKey)
	if err != nil {
		abortTx(tx, w)
		return fmt.Errorf("read local scope metadata: %w", err)
	}
	meta := Meta{Scope: s.scope}
	if found {
		if err := json.Unmarshal([]byte(value.String()), &meta); err != nil {
			abortTx(tx, w)
			return fmt.Errorf("decode local scope metadata: %w", err)
		}
		if meta.Scope != s.scope {
			abortTx(tx, w)
			return &ScopeMismatchError{Expected: s.scope, Actual: meta.Scope}
		}
	} else {
		encoded, err := marshalRecord(meta)
		if err != nil {
			abortTx(tx, w)
			return err
		}
		if err := putInTx(tx, idbStoreMeta, encoded, idbMetaKey); err != nil {
			abortTx(tx, w)
			return err
		}
	}
	if err := w.wait(); err != nil {
		return fmt.Errorf("store local scope metadata: %w", err)
	}
	return nil
}

func (s *IDBStore) readyLocked() error {
	if !s.opened {
		return fmt.Errorf("store is not open: %w", ErrInvalid)
	}
	return nil
}

// getInTx reads one record inside an open transaction.
func getInTx(tx js.Value, storeName string, key any) (js.Value, bool, error) {
	store, err := txStore(tx, storeName)
	if err != nil {
		return js.Undefined(), false, err
	}
	req, err := callValue(func() js.Value { return store.Call("get", key) })
	if err != nil {
		return js.Undefined(), false, err
	}
	result, err := awaitRequest(req)
	if err != nil {
		return js.Undefined(), false, err
	}
	if result.IsUndefined() || result.IsNull() {
		return js.Undefined(), false, nil
	}
	return result, true, nil
}

// putInTx places a put inside an open transaction. The caller observes the
// outcome through the transaction waiter, never before commit.
func putInTx(tx js.Value, storeName string, value, key any) error {
	store, err := txStore(tx, storeName)
	if err != nil {
		return err
	}
	_, err = callValue(func() js.Value { store.Call("put", value, key); return js.Undefined() })
	return err
}

// readOne fetches one record in its own readonly transaction.
func (s *IDBStore) readOne(storeName string, key any) (js.Value, bool, error) {
	tx, w, err := s.newTx([]string{storeName}, "readonly")
	if err != nil {
		return js.Undefined(), false, err
	}
	value, found, err := getInTx(tx, storeName, key)
	if err != nil {
		abortTx(tx, w)
		return js.Undefined(), false, err
	}
	if err := w.wait(); err != nil {
		return js.Undefined(), false, err
	}
	return value, found, nil
}

// readJSON fetches and decodes one JSON-string record.
func (s *IDBStore) readJSON(storeName string, key any, out any) (bool, error) {
	value, found, err := s.readOne(storeName, key)
	if err != nil || !found {
		return found, err
	}
	if err := json.Unmarshal([]byte(value.String()), out); err != nil {
		return true, fmt.Errorf("decode %s record: %w", storeName, err)
	}
	return true, nil
}

func marshalRecord(v any) (string, error) {
	blob, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode record: %w", err)
	}
	return string(blob), nil
}

// getCountersInTx reads the sequence counters (zero when absent).
func getCountersInTx(tx js.Value) (counters, error) {
	value, found, err := getInTx(tx, idbStoreMeta, idbCountersKey)
	if err != nil || !found {
		return counters{}, err
	}
	var c counters
	if err := json.Unmarshal([]byte(value.String()), &c); err != nil {
		return counters{}, fmt.Errorf("decode counters record: %w", err)
	}
	return c, nil
}

func putCountersInTx(tx js.Value, c counters) error {
	encoded, err := marshalRecord(c)
	if err != nil {
		return err
	}
	return putInTx(tx, idbStoreMeta, encoded, idbCountersKey)
}

// logRecord builds the stored log object. The indexed fields sit alongside
// the JSON payload so the unique changeId index can enforce dedupe inside
// the database itself.
func logRecord(entry LogEntry) (js.Value, error) {
	encoded, err := marshalRecord(entry)
	if err != nil {
		return js.Undefined(), err
	}
	obj := js.Global().Get("Object").New()
	obj.Set("changeId", entry.ChangeID)
	obj.Set("workspaceId", entry.WorkspaceID)
	obj.Set("serverSeq", float64(entry.ServerSeq))
	obj.Set("data", encoded)
	return obj, nil
}

func decodeLogRecord(value js.Value, key uint64) (LogEntry, error) {
	var entry LogEntry
	data := value.Get("data")
	if data.Type() != js.TypeString {
		return entry, fmt.Errorf("decode log record %d: %w", key, ErrInvalid)
	}
	if err := json.Unmarshal([]byte(data.String()), &entry); err != nil {
		return entry, fmt.Errorf("decode log record %d: %w", key, err)
	}
	entry.LocalSeq = key // the record key is authoritative
	entry.Sealed = clone(entry.Sealed)
	return entry, nil
}

// readLogInTx returns every log entry in localSeq order within tx.
func readLogInTx(tx js.Value) ([]LogEntry, error) {
	store, err := txStore(tx, idbStoreLog)
	if err != nil {
		return nil, err
	}
	valuesReq, err := callValue(func() js.Value { return store.Call("getAll") })
	if err != nil {
		return nil, err
	}
	keysReq, err := callValue(func() js.Value { return store.Call("getAllKeys") })
	if err != nil {
		return nil, err
	}
	values, err := awaitRequest(valuesReq)
	if err != nil {
		return nil, err
	}
	keys, err := awaitRequest(keysReq)
	if err != nil {
		return nil, err
	}
	n := values.Length()
	if keys.Length() != n {
		return nil, fmt.Errorf("read log: %d values for %d keys: %w", n, keys.Length(), ErrInvalid)
	}
	out := make([]LogEntry, 0, n)
	for i := 0; i < n; i++ {
		entry, err := decodeLogRecord(values.Index(i), uint64(keys.Index(i).Float()))
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

// readAllLog returns every log entry in its own readonly transaction.
func (s *IDBStore) readAllLog() ([]LogEntry, error) {
	tx, w, err := s.newTx([]string{idbStoreLog}, "readonly")
	if err != nil {
		return nil, err
	}
	entries, err := readLogInTx(tx)
	if err != nil {
		abortTx(tx, w)
		return nil, err
	}
	if err := w.wait(); err != nil {
		return nil, err
	}
	return entries, nil
}

// migrateLegacyLocked converts a pre-SV3a.1 meta/state checkpoint into
// per-store records in one strict transaction and deletes the checkpoint
// in the same transaction. Any failure leaves the checkpoint in place for
// a retry; nothing is half-migrated and nothing is silently dropped. The
// caller holds s.mu and s.db is set.
func (s *IDBStore) migrateLegacyLocked() error {
	var legacy diskState
	found, err := s.readJSON(idbStoreMeta, idbLegacyState, &legacy)
	if err != nil {
		return fmt.Errorf("read legacy local state: %w", err)
	}
	if !found {
		return nil
	}
	if legacy.Scope != s.scope {
		return &ScopeMismatchError{Expected: s.scope, Actual: legacy.Scope}
	}
	tx, w, err := s.newTx(idbAllStores, "readwrite")
	if err != nil {
		return err
	}
	fail := func(err error) error {
		abortTx(tx, w)
		return fmt.Errorf("migrate legacy local state: %w", err)
	}
	legacy.Meta.Scope = legacy.Scope
	metaJSON, err := marshalRecord(legacy.Meta)
	if err != nil {
		return fail(err)
	}
	if err := putInTx(tx, idbStoreMeta, metaJSON, idbMetaKey); err != nil {
		return fail(err)
	}
	c := counters{NextSeq: legacy.NextSeq, QuarantineSeq: uint64(len(legacy.Quarantine))}
	for _, entry := range legacy.Log {
		if entry.LocalSeq > c.NextSeq {
			c.NextSeq = entry.LocalSeq
		}
	}
	if err := putCountersInTx(tx, c); err != nil {
		return fail(err)
	}
	if legacy.Keyring != nil {
		encoded, err := marshalRecord(keyringRecord{Version: legacy.KeyringVersion, Blob: clone(legacy.Keyring)})
		if err != nil {
			return fail(err)
		}
		if err := putInTx(tx, idbStoreKeyring, encoded, idbKeyringKey); err != nil {
			return fail(err)
		}
	}
	logStore, err := txStore(tx, idbStoreLog)
	if err != nil {
		return fail(err)
	}
	for _, entry := range legacy.Log {
		record, err := logRecord(entry)
		if err != nil {
			return fail(err)
		}
		if _, err := callValue(func() js.Value {
			logStore.Call("add", record, float64(entry.LocalSeq))
			return js.Undefined()
		}); err != nil {
			return fail(err)
		}
	}
	for workspaceID, snap := range legacy.Snapshots {
		encoded, err := marshalRecord(snap)
		if err != nil {
			return fail(err)
		}
		if err := putInTx(tx, idbStoreSnapshot, encoded, workspaceID); err != nil {
			return fail(err)
		}
	}
	for key, sealed := range legacy.Drafts {
		encoded, err := marshalRecord(sealed)
		if err != nil {
			return fail(err)
		}
		if err := putInTx(tx, idbStoreDrafts, encoded, key); err != nil {
			return fail(err)
		}
	}
	quarantineStore, err := txStore(tx, idbStoreQuarantine)
	if err != nil {
		return fail(err)
	}
	for i, entry := range legacy.Quarantine {
		encoded, err := marshalRecord(entry)
		if err != nil {
			return fail(err)
		}
		if _, err := callValue(func() js.Value {
			quarantineStore.Call("add", encoded, float64(i+1))
			return js.Undefined()
		}); err != nil {
			return fail(err)
		}
	}
	metaStore, err := txStore(tx, idbStoreMeta)
	if err != nil {
		return fail(err)
	}
	if _, err := callValue(func() js.Value {
		metaStore.Call("delete", idbLegacyState)
		return js.Undefined()
	}); err != nil {
		return fail(err)
	}
	if err := w.wait(); err != nil {
		return fmt.Errorf("migrate legacy local state: %w", err)
	}
	return nil
}

// Meta returns the scope metadata, or a zero Meta when none was stored.
func (s *IDBStore) Meta() (Meta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return Meta{}, err
	}
	var meta Meta
	if _, err := s.readJSON(idbStoreMeta, idbMetaKey, &meta); err != nil {
		return Meta{}, err
	}
	return meta, nil
}

// PutMeta stores metadata with a version one higher than the stored
// version, read and written in the same transaction so two tabs cannot
// claim the same version.
func (s *IDBStore) PutMeta(v Meta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return err
	}
	v.Scope = s.scope
	tx, w, err := s.newTx([]string{idbStoreMeta}, "readwrite")
	if err != nil {
		return err
	}
	current, found, err := getInTx(tx, idbStoreMeta, idbMetaKey)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if found {
		var stored Meta
		if err := json.Unmarshal([]byte(current.String()), &stored); err != nil {
			abortTx(tx, w)
			return fmt.Errorf("decode meta record: %w", err)
		}
		v.Version = stored.Version + 1
	} else {
		v.Version = 1
	}
	encoded, err := marshalRecord(v)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if err := putInTx(tx, idbStoreMeta, encoded, idbMetaKey); err != nil {
		abortTx(tx, w)
		return err
	}
	return w.wait()
}

// Keyring returns the stored opaque keyring blob.
func (s *IDBStore) Keyring() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return nil, err
	}
	var record keyringRecord
	found, err := s.readJSON(idbStoreKeyring, idbKeyringKey, &record)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	return clone(record.Blob), nil
}

// PutKeyring compare-and-swaps the keyring blob: the version check and the
// write happen in one transaction, so a racing tab cannot slip between
// them. A version mismatch aborts and leaves the stored blob untouched.
func (s *IDBStore) PutKeyring(blob []byte, expectedVersion uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return err
	}
	tx, w, err := s.newTx([]string{idbStoreKeyring}, "readwrite")
	if err != nil {
		return err
	}
	current, found, err := getInTx(tx, idbStoreKeyring, idbKeyringKey)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	var version uint64
	if found {
		var stored keyringRecord
		if err := json.Unmarshal([]byte(current.String()), &stored); err != nil {
			abortTx(tx, w)
			return fmt.Errorf("decode keyring record: %w", err)
		}
		version = stored.Version
	}
	if version != expectedVersion {
		abortTx(tx, w)
		return ErrConflict
	}
	encoded, err := marshalRecord(keyringRecord{Version: version + 1, Blob: clone(blob)})
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if err := putInTx(tx, idbStoreKeyring, encoded, idbKeyringKey); err != nil {
		abortTx(tx, w)
		return err
	}
	return w.wait()
}

// Append durably adds one pending log entry and returns its localSeq only
// after the transaction commits. A duplicate ChangeID is rejected by the
// unique index with ErrConflict.
func (s *IDBStore) Append(workspaceID, changeID string, sealed []byte, created changes.HLC) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return 0, err
	}
	if workspaceID == "" || changeID == "" || len(sealed) == 0 {
		return 0, ErrInvalid
	}
	return s.appendLocked(workspaceID, changeID, sealed, 0, created, false)
}

// AppendRemote durably adds an already-acknowledged entry from the server,
// deduplicating by ChangeID: an identical repeat returns the existing
// localSeq, a conflicting repeat fails with ErrConflict.
func (s *IDBStore) AppendRemote(workspaceID, changeID string, sealed []byte, serverSeq uint64, created changes.HLC) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return 0, err
	}
	if workspaceID == "" || changeID == "" || len(sealed) == 0 || serverSeq == 0 {
		return 0, ErrInvalid
	}
	return s.appendLocked(workspaceID, changeID, sealed, serverSeq, created, true)
}

// appendLocked is the shared append path. The sequence counter is read,
// the entry inserted with add() at the new sequence, and the counter
// stored — all in one readwrite transaction. IndexedDB serializes such
// transactions across tabs, and an abort rolls back entry and counter
// together, so a failed append consumes no sequence and leaves no record.
func (s *IDBStore) appendLocked(workspaceID, changeID string, sealed []byte, serverSeq uint64, created changes.HLC, remote bool) (uint64, error) {
	tx, w, err := s.newTx([]string{idbStoreMeta, idbStoreLog}, "readwrite")
	if err != nil {
		return 0, err
	}
	logStore, err := txStore(tx, idbStoreLog)
	if err != nil {
		abortTx(tx, w)
		return 0, err
	}
	if remote {
		existing, existingSeq, found, err := findByChangeID(logStore, changeID)
		if err != nil {
			abortTx(tx, w)
			return 0, err
		}
		if found {
			if existing.WorkspaceID != workspaceID || existing.ServerSeq != serverSeq || !bytes.Equal(existing.Sealed, sealed) {
				abortTx(tx, w)
				return 0, ErrConflict
			}
			// Identical repeat: nothing to write. Let the (read-only in
			// effect) transaction commit, then report the existing seq.
			if err := w.wait(); err != nil {
				return 0, err
			}
			return existingSeq, nil
		}
	}
	c, err := getCountersInTx(tx)
	if err != nil {
		abortTx(tx, w)
		return 0, err
	}
	seq := c.NextSeq + 1
	entry := LogEntry{
		LocalSeq:    seq,
		ChangeID:    changeID,
		WorkspaceID: workspaceID,
		Sealed:      clone(sealed),
		ServerSeq:   serverSeq,
		CreatedHLC:  created,
	}
	record, err := logRecord(entry)
	if err != nil {
		abortTx(tx, w)
		return 0, err
	}
	if _, err := callValue(func() js.Value {
		logStore.Call("add", record, float64(seq))
		return js.Undefined()
	}); err != nil {
		abortTx(tx, w)
		return 0, err
	}
	c.NextSeq = seq
	if err := putCountersInTx(tx, c); err != nil {
		abortTx(tx, w)
		return 0, err
	}
	if err := w.wait(); err != nil {
		if storageErrorName(err) == "ConstraintError" {
			return 0, fmt.Errorf("append change %q: %w", changeID, ErrConflict)
		}
		return 0, err
	}
	return seq, nil
}

// findByChangeID looks up a log entry through the unique changeId index
// inside the caller's transaction.
func findByChangeID(logStore js.Value, changeID string) (LogEntry, uint64, bool, error) {
	index, err := callValue(func() js.Value { return logStore.Call("index", idbLogChangeIdx) })
	if err != nil {
		return LogEntry{}, 0, false, err
	}
	valueReq, err := callValue(func() js.Value { return index.Call("get", changeID) })
	if err != nil {
		return LogEntry{}, 0, false, err
	}
	keyReq, err := callValue(func() js.Value { return index.Call("getKey", changeID) })
	if err != nil {
		return LogEntry{}, 0, false, err
	}
	value, err := awaitRequest(valueReq)
	if err != nil {
		return LogEntry{}, 0, false, err
	}
	key, err := awaitRequest(keyReq)
	if err != nil {
		return LogEntry{}, 0, false, err
	}
	if value.IsUndefined() || value.IsNull() {
		return LogEntry{}, 0, false, nil
	}
	seq := uint64(key.Float())
	entry, err := decodeLogRecord(value, seq)
	if err != nil {
		return LogEntry{}, 0, false, err
	}
	return entry, seq, true, nil
}

// Pending returns the oldest unacknowledged entries, up to limit.
func (s *IDBStore) Pending(limit int) ([]LogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return []LogEntry{}, nil
	}
	entries, err := s.readAllLog()
	if err != nil {
		return nil, err
	}
	out := []LogEntry{}
	for _, entry := range entries {
		if entry.ServerSeq == 0 {
			out = append(out, entry)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

// Since returns every entry with a localSeq greater than the given one, in
// localSeq order.
func (s *IDBStore) Since(localSeq uint64) ([]LogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return nil, err
	}
	entries, err := s.readAllLog()
	if err != nil {
		return nil, err
	}
	out := []LogEntry{}
	for _, entry := range entries {
		if entry.LocalSeq > localSeq {
			out = append(out, entry)
		}
	}
	return out, nil
}

// MarkAcked records the server sequence for one entry. The read, conflict
// check and write share one transaction; an abort changes nothing.
func (s *IDBStore) MarkAcked(localSeq, serverSeq uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return err
	}
	if serverSeq == 0 {
		return ErrInvalid
	}
	tx, w, err := s.newTx([]string{idbStoreLog}, "readwrite")
	if err != nil {
		return err
	}
	value, found, err := getInTx(tx, idbStoreLog, float64(localSeq))
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if !found {
		abortTx(tx, w)
		return ErrNotFound
	}
	entry, err := decodeLogRecord(value, localSeq)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if entry.ServerSeq != 0 && entry.ServerSeq != serverSeq {
		abortTx(tx, w)
		return ErrConflict
	}
	entry.ServerSeq = serverSeq
	record, err := logRecord(entry)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if err := putInTx(tx, idbStoreLog, record, float64(localSeq)); err != nil {
		abortTx(tx, w)
		return err
	}
	return w.wait()
}

// LatestSnapshot returns the stored snapshot for a workspace.
func (s *IDBStore) LatestSnapshot(workspaceID string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return Snapshot{}, err
	}
	var snap Snapshot
	found, err := s.readJSON(idbStoreSnapshot, workspaceID, &snap)
	if err != nil {
		return Snapshot{}, err
	}
	if !found {
		return Snapshot{}, ErrNotFound
	}
	snap.Sealed = clone(snap.Sealed)
	return snap, nil
}

// Compact writes the workspace snapshot and deletes the covered,
// acknowledged log entries in one transaction. Pending entries are never
// deleted, and an abort leaves snapshot and log exactly as they were.
func (s *IDBStore) Compact(workspaceID string, sealed []byte, coversLocalSeq, coversServerSeq uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return err
	}
	if workspaceID == "" || len(sealed) == 0 {
		return ErrInvalid
	}
	tx, w, err := s.newTx([]string{idbStoreLog, idbStoreSnapshot}, "readwrite")
	if err != nil {
		return err
	}
	entries, err := readLogInTx(tx)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	snap := Snapshot{
		WorkspaceID:     workspaceID,
		CoversLocalSeq:  coversLocalSeq,
		CoversServerSeq: coversServerSeq,
		Sealed:          clone(sealed),
	}
	encoded, err := marshalRecord(snap)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if err := putInTx(tx, idbStoreSnapshot, encoded, workspaceID); err != nil {
		abortTx(tx, w)
		return err
	}
	logStore, err := txStore(tx, idbStoreLog)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	for _, entry := range entries {
		if entry.WorkspaceID != workspaceID || entry.LocalSeq > coversLocalSeq || entry.ServerSeq == 0 {
			continue
		}
		seq := entry.LocalSeq
		if _, err := callValue(func() js.Value {
			logStore.Call("delete", float64(seq))
			return js.Undefined()
		}); err != nil {
			abortTx(tx, w)
			return err
		}
	}
	return w.wait()
}

// PutDraft durably stores one sealed draft under key.
func (s *IDBStore) PutDraft(key string, sealed []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return err
	}
	if key == "" {
		return ErrInvalid
	}
	encoded, err := marshalRecord(clone(sealed))
	if err != nil {
		return err
	}
	tx, w, err := s.newTx([]string{idbStoreDrafts}, "readwrite")
	if err != nil {
		return err
	}
	if err := putInTx(tx, idbStoreDrafts, encoded, key); err != nil {
		abortTx(tx, w)
		return err
	}
	return w.wait()
}

// GetDraft returns the sealed draft stored under key.
func (s *IDBStore) GetDraft(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return nil, err
	}
	var sealed []byte
	found, err := s.readJSON(idbStoreDrafts, key, &sealed)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	return clone(sealed), nil
}

// DeleteDraft removes the draft stored under key, if any.
func (s *IDBStore) DeleteDraft(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return err
	}
	tx, w, err := s.newTx([]string{idbStoreDrafts}, "readwrite")
	if err != nil {
		return err
	}
	store, err := txStore(tx, idbStoreDrafts)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	if _, err := callValue(func() js.Value {
		store.Call("delete", key)
		return js.Undefined()
	}); err != nil {
		abortTx(tx, w)
		return err
	}
	return w.wait()
}

// Quarantine durably records unreadable raw bytes with the reason they
// were rejected. Its sequence comes from the shared counters record in the
// same transaction, so entries from two tabs never collide.
func (s *IDBStore) Quarantine(raw []byte, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return err
	}
	if reason == "" {
		return ErrInvalid
	}
	tx, w, err := s.newTx([]string{idbStoreMeta, idbStoreQuarantine}, "readwrite")
	if err != nil {
		return err
	}
	c, err := getCountersInTx(tx)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	c.QuarantineSeq++
	encoded, err := marshalRecord(quarantineEntry{Raw: clone(raw), Reason: reason, At: time.Now().UTC()})
	if err != nil {
		abortTx(tx, w)
		return err
	}
	store, err := txStore(tx, idbStoreQuarantine)
	if err != nil {
		abortTx(tx, w)
		return err
	}
	seq := c.QuarantineSeq
	if _, err := callValue(func() js.Value {
		store.Call("add", encoded, float64(seq))
		return js.Undefined()
	}); err != nil {
		abortTx(tx, w)
		return err
	}
	if err := putCountersInTx(tx, c); err != nil {
		abortTx(tx, w)
		return err
	}
	return w.wait()
}

// IsQuarantined reports whether raw bytes were already durably recorded in
// quarantine. Boot uses this to avoid retrying a rejected log entry forever.
func (s *IDBStore) IsQuarantined(raw []byte) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return false, err
	}
	tx, w, err := s.newTx([]string{idbStoreQuarantine}, "readonly")
	if err != nil {
		return false, err
	}
	store, err := txStore(tx, idbStoreQuarantine)
	if err != nil {
		abortTx(tx, w)
		return false, err
	}
	req, err := callValue(func() js.Value { return store.Call("getAll") })
	if err != nil {
		abortTx(tx, w)
		return false, err
	}
	values, err := awaitRequest(req)
	if err != nil {
		abortTx(tx, w)
		return false, err
	}
	for i := 0; i < values.Length(); i++ {
		var entry quarantineEntry
		if err := json.Unmarshal([]byte(values.Index(i).String()), &entry); err != nil {
			abortTx(tx, w)
			return false, fmt.Errorf("decode quarantine entry: %w", err)
		}
		if bytes.Equal(entry.Raw, raw) {
			if err := w.wait(); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	if err := w.wait(); err != nil {
		return false, err
	}
	return false, nil
}

// RequestPersistence asks the browser to keep this origin's storage
// persistent via navigator.storage.persist. Browsers without the API
// report not granted.
func (s *IDBStore) RequestPersistence() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.readyLocked(); err != nil {
		return false, err
	}
	storage := js.Global().Get("navigator").Get("storage")
	if storage.IsUndefined() || storage.IsNull() || storage.Get("persist").IsUndefined() {
		return false, nil
	}
	promise, err := callValue(func() js.Value { return storage.Call("persist") })
	if err != nil {
		return false, fmt.Errorf("request persistent storage: %w", err)
	}
	v, err := awaitPromise(promise)
	if err != nil {
		return false, fmt.Errorf("request persistent storage: %w", err)
	}
	return v.Bool(), nil
}
