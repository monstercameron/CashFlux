// SPDX-License-Identifier: MIT
//go:build js && wasm

package localdb_test

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/changes"
	"github.com/monstercameron/CashFlux/internal/keyring"
	"github.com/monstercameron/CashFlux/internal/localdb"
	"github.com/monstercameron/CashFlux/internal/localdb/storetest"
)

// fakeIDBJS is the test-only in-memory IndexedDB shim. Node (the wasm test
// lane) has no indexedDB, so the tests install this shim and run the real
// IDBStore code against it. In a browser harness the real IndexedDB is used
// untouched and only the failure-injection tests skip.
//
//go:embed testdata/fake_indexeddb.js
var fakeIDBJS string

var scopeCounter int

// testScope returns a fresh scope (hence a fresh database) per test.
func testScope() keyring.Scope {
	scopeCounter++
	return keyring.Scope{ServerID: "srv-test", AccountID: fmt.Sprintf("acct-%d", scopeCounter)}
}

// setupIDB installs the shim when the runtime has no IndexedDB and resets it
// so each test starts from an empty set of databases. It reports whether the
// shim (with its failure-injection controls) is in use.
func setupIDB(t *testing.T) bool {
	t.Helper()
	idb := js.Global().Get("indexedDB")
	if idb.IsUndefined() || idb.IsNull() {
		js.Global().Call("eval", fakeIDBJS)
	}
	if fake := js.Global().Get("__fakeIDB"); fake.Truthy() {
		fake.Call("reset")
		return true
	}
	return false
}

func requireFake(t *testing.T, fake bool) {
	t.Helper()
	if !fake {
		t.Skip("failure injection needs the test IndexedDB shim; this runtime has a real IndexedDB")
	}
}

func failNextRequest(name string) { js.Global().Get("__fakeIDB").Call("failNextRequest", name) }
func failNextWrite(name string)   { js.Global().Get("__fakeIDB").Call("failNextWrite", name) }
func failNextOpen(name string)    { js.Global().Get("__fakeIDB").Call("failNextOpen", name) }
func failNextTransaction(name string) {
	js.Global().Get("__fakeIDB").Call("failNextTransaction", name)
}

func awaitRequest(t *testing.T, req js.Value) js.Value {
	t.Helper()
	ch := make(chan struct{})
	var result js.Value
	var failure string
	var once sync.Once
	ok := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		once.Do(func() { result = req.Get("result"); close(ch) })
		return nil
	})
	bad := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		once.Do(func() {
			if e := req.Get("error"); e.Truthy() {
				failure = e.Get("name").String() + ": " + e.Get("message").String()
			} else {
				failure = "request failed"
			}
			close(ch)
		})
		return nil
	})
	req.Set("onsuccess", ok)
	req.Set("onerror", bad)
	<-ch
	ok.Release()
	bad.Release()
	if failure != "" {
		t.Fatalf("IndexedDB request failed: %s", failure)
	}
	return result
}

func awaitPromise(t *testing.T, p js.Value) {
	t.Helper()
	ch := make(chan struct{})
	var failure string
	var once sync.Once
	ok := js.FuncOf(func(_ js.Value, _ []js.Value) any { once.Do(func() { close(ch) }); return nil })
	bad := js.FuncOf(func(_ js.Value, a []js.Value) any {
		once.Do(func() { failure = a[0].String(); close(ch) })
		return nil
	})
	p.Call("then", ok).Call("catch", bad)
	<-ch
	ok.Release()
	bad.Release()
	if failure != "" {
		t.Fatalf("promise rejected: %s", failure)
	}
}

func deleteDB(t *testing.T, name string) {
	t.Helper()
	awaitRequest(t, js.Global().Get("indexedDB").Call("deleteDatabase", name))
}

func openStore(t *testing.T, scope keyring.Scope) *localdb.IDBStore {
	t.Helper()
	s := localdb.NewIDBStore()
	if err := s.Open(scope); err != nil {
		t.Fatalf("open IDBStore: %v", err)
	}
	return s
}

// TestIDBStoreConformance runs the shared Store contract suite against the
// IndexedDB adapter (real IndexedDB in a browser, the shim under Node).
func TestIDBStoreConformance(t *testing.T) {
	setupIDB(t)
	deleteDB(t, localdb.DBName(keyring.LocalScope("acct")))
	storetest.Conformance(t, func() localdb.Store { return localdb.NewIDBStore() })
}

// TestIDBTwoInstancesConcurrentAppend proves two store instances (two tabs)
// appending at the same time never overwrite each other's log entries: every
// append gets its own localSeq record in the shared database.
func TestIDBTwoInstancesConcurrentAppend(t *testing.T) {
	setupIDB(t)
	scope := testScope()
	a := openStore(t, scope)
	b := openStore(t, scope)

	const perInstance = 10
	var wg sync.WaitGroup
	errs := make(chan error, 2*perInstance)
	appendAll := func(s *localdb.IDBStore, prefix string) {
		defer wg.Done()
		for i := 0; i < perInstance; i++ {
			id := fmt.Sprintf("%s-%d", prefix, i)
			if _, err := s.Append("ws", id, []byte("cipher-"+id), changes.HLC{WallMillis: int64(i + 1)}); err != nil {
				errs <- fmt.Errorf("append %s: %w", id, err)
			}
		}
	}
	wg.Add(2)
	go appendAll(a, "a")
	go appendAll(b, "b")
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		t.FailNow()
	}

	// A third, fresh instance must see every entry from both writers.
	c := openStore(t, scope)
	entries, err := c.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2*perInstance {
		t.Fatalf("entries = %d, want %d: %+v", len(entries), 2*perInstance, entries)
	}
	seqs := map[uint64]string{}
	ids := map[string]uint64{}
	var prev uint64
	for _, e := range entries {
		if e.LocalSeq <= prev {
			t.Errorf("entries out of order: seq %d after %d", e.LocalSeq, prev)
		}
		prev = e.LocalSeq
		if other, dup := seqs[e.LocalSeq]; dup {
			t.Errorf("localSeq %d shared by %q and %q", e.LocalSeq, other, e.ChangeID)
		}
		seqs[e.LocalSeq] = e.ChangeID
		ids[e.ChangeID] = e.LocalSeq
		if string(e.Sealed) != "cipher-"+e.ChangeID {
			t.Errorf("entry %q sealed = %q", e.ChangeID, e.Sealed)
		}
	}
	for i := 0; i < perInstance; i++ {
		for _, prefix := range []string{"a", "b"} {
			if _, ok := ids[fmt.Sprintf("%s-%d", prefix, i)]; !ok {
				t.Errorf("missing change %s-%d", prefix, i)
			}
		}
	}
	// The first instance also sees the second instance's entries on re-read.
	if entries, err = a.Since(0); err != nil || len(entries) != 2*perInstance {
		t.Fatalf("instance a re-read: %d entries, err=%v", len(entries), err)
	}
}

// TestIDBDuplicateChangeAcrossInstances: the unique ChangeID index rejects
// a second tab's duplicate append without eating its next sequence.
func TestIDBDuplicateChangeAcrossInstances(t *testing.T) {
	setupIDB(t)
	scope := testScope()
	a := openStore(t, scope)
	b := openStore(t, scope)
	if _, err := a.Append("ws", "c1", []byte("cipher1"), changes.HLC{WallMillis: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Append("ws", "c1", []byte("cipher1"), changes.HLC{WallMillis: 1}); !errors.Is(err, localdb.ErrConflict) {
		t.Fatalf("duplicate append err = %v, want ErrConflict", err)
	}
	seq, err := b.Append("ws", "c2", []byte("cipher2"), changes.HLC{WallMillis: 2})
	if err != nil {
		t.Fatal(err)
	}
	if seq != 2 {
		t.Fatalf("seq after rejected duplicate = %d, want 2 (rejected append must not consume a sequence)", seq)
	}
	entries, err := a.Since(0)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %+v err=%v", entries, err)
	}
}

// TestIDBQuotaFailureNotPublished: a write whose transaction aborts with
// QuotaExceededError must leave no trace — not in reads afterwards, and not
// as a phantom ChangeID that blocks a retry once storage recovers.
func TestIDBQuotaFailureNotPublished(t *testing.T) {
	requireFake(t, setupIDB(t))
	s := openStore(t, testScope())
	if _, err := s.Append("ws", "c1", []byte("cipher1"), changes.HLC{WallMillis: 1}); err != nil {
		t.Fatal(err)
	}
	failNextWrite("QuotaExceededError")
	_, err := s.Append("ws", "c2", []byte("cipher2"), changes.HLC{WallMillis: 2})
	var quota *localdb.QuotaError
	if !errors.As(err, &quota) {
		t.Fatalf("quota append err = %v (%T), want *QuotaError", err, err)
	}
	entries, err := s.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ChangeID != "c1" {
		t.Fatalf("failed append was published: %+v", entries)
	}
	// The same ChangeID can be appended once storage works again, and it
	// takes the very next sequence: the aborted attempt consumed nothing.
	seq, err := s.Append("ws", "c2", []byte("cipher2"), changes.HLC{WallMillis: 2})
	if err != nil {
		t.Fatalf("retry after quota failure: %v", err)
	}
	if seq != 2 {
		t.Fatalf("retry seq = %d, want 2", seq)
	}
}

// TestIDBStrictTransactionFailureDoesNotRetry proves constructor errors are
// returned with their DOM name and never trigger a weaker transaction retry.
func TestIDBStrictTransactionFailureDoesNotRetry(t *testing.T) {
	requireFake(t, setupIDB(t))
	s := openStore(t, testScope())
	fake := js.Global().Get("__fakeIDB")
	before := fake.Call("transactionCount").Int()
	failNextTransaction("TypeError")
	err := s.PutMeta(localdb.Meta{DeviceID: "must-not-be-acknowledged"})
	var storage *localdb.StorageError
	if !errors.As(err, &storage) || storage.Name != "TypeError" {
		t.Fatalf("PutMeta error = %v (%T), want *StorageError named TypeError", err, err)
	}
	if got := fake.Call("transactionCount").Int() - before; got != 1 {
		t.Fatalf("transaction() calls = %d, want exactly 1 (no fallback)", got)
	}
}

// TestIDBErrorMappingLeavesStateUnchanged: abort, security, unsupported and
// quota failures map to distinct typed errors carrying the DOMException
// name, and none of them mutate the stored state.
func TestIDBErrorMappingLeavesStateUnchanged(t *testing.T) {
	requireFake(t, setupIDB(t))
	s := openStore(t, testScope())
	if err := s.PutMeta(localdb.Meta{DeviceID: "baseline"}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		domName  string
		sentinel error // nil for quota
	}{
		{domName: "AbortError", sentinel: localdb.ErrStorageAborted},
		{domName: "SecurityError", sentinel: localdb.ErrStorageSecurity},
		{domName: "NotSupportedError", sentinel: localdb.ErrStorageUnsupported},
		{domName: "QuotaExceededError"},
		{domName: "UnknownError"},
	}
	for _, tc := range cases {
		t.Run(tc.domName, func(t *testing.T) {
			failNextRequest(tc.domName)
			err := s.PutMeta(localdb.Meta{DeviceID: "should-not-stick"})
			if err == nil {
				t.Fatal("expected error from injected failure")
			}
			var quota *localdb.QuotaError
			isQuota := errors.As(err, &quota)
			if tc.domName == "QuotaExceededError" {
				if !isQuota {
					t.Fatalf("err = %v (%T), want *QuotaError", err, err)
				}
				if quota.Name != tc.domName {
					t.Errorf("QuotaError.Name = %q, want %q", quota.Name, tc.domName)
				}
			} else {
				if isQuota {
					t.Fatalf("err = %v, must not be *QuotaError", err)
				}
				var storage *localdb.StorageError
				if !errors.As(err, &storage) {
					t.Fatalf("err = %v (%T), want *StorageError", err, err)
				}
				if storage.Name != tc.domName {
					t.Errorf("StorageError.Name = %q, want %q", storage.Name, tc.domName)
				}
				if tc.sentinel != nil && !errors.Is(err, tc.sentinel) {
					t.Errorf("errors.Is(err, %v) = false for %v", tc.sentinel, err)
				}
			}
			meta, mErr := s.Meta()
			if mErr != nil {
				t.Fatal(mErr)
			}
			if meta.DeviceID != "baseline" {
				t.Fatalf("failed PutMeta was published: %+v", meta)
			}
		})
	}
}

// TestIDBCompactFailureLeavesLogIntact: a compaction whose transaction
// aborts must neither delete log entries nor publish a snapshot.
func TestIDBCompactFailureLeavesLogIntact(t *testing.T) {
	requireFake(t, setupIDB(t))
	s := openStore(t, testScope())
	seq, err := s.Append("ws", "c1", []byte("cipher1"), changes.HLC{WallMillis: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkAcked(seq, 7); err != nil {
		t.Fatal(err)
	}
	failNextWrite("QuotaExceededError")
	err = s.Compact("ws", []byte("snapshot"), seq, 7)
	var quota *localdb.QuotaError
	if !errors.As(err, &quota) {
		t.Fatalf("compact err = %v (%T), want *QuotaError", err, err)
	}
	entries, err := s.Since(0)
	if err != nil || len(entries) != 1 || entries[0].ChangeID != "c1" {
		t.Fatalf("log after failed compact = %+v err=%v", entries, err)
	}
	if _, err := s.LatestSnapshot("ws"); !errors.Is(err, localdb.ErrNotFound) {
		t.Fatalf("snapshot after failed compact err = %v, want ErrNotFound", err)
	}
	// And a working compact still works afterwards.
	if err := s.Compact("ws", []byte("snapshot"), seq, 7); err != nil {
		t.Fatal(err)
	}
	if entries, err = s.Since(0); err != nil || len(entries) != 0 {
		t.Fatalf("log after compact = %+v err=%v", entries, err)
	}
	if snap, err := s.LatestSnapshot("ws"); err != nil || string(snap.Sealed) != "snapshot" {
		t.Fatalf("snapshot = %+v err=%v", snap, err)
	}
}

// dumpDB returns the shim's view of a database: store name to [key, value]
// pairs. It lets tests prove records land in their own object stores.
func dumpDB(t *testing.T, dbName string) map[string][][2]json.RawMessage {
	t.Helper()
	raw := js.Global().Get("__fakeIDB").Call("dump", dbName).String()
	out := map[string][][2]json.RawMessage{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode dump: %v (%s)", err, raw)
	}
	return out
}

func dumpKeys(pairs [][2]json.RawMessage) []string {
	keys := make([]string, 0, len(pairs))
	for _, p := range pairs {
		var k string
		_ = json.Unmarshal(p[0], &k)
		keys = append(keys, k)
	}
	return keys
}

// TestIDBPerStoreRecords: each record kind lives in its own object store as
// its own record — in particular log entries are individual records keyed by
// localSeq, and there is no whole-database blob hiding in meta.
func TestIDBPerStoreRecords(t *testing.T) {
	requireFake(t, setupIDB(t))
	scope := testScope()
	s := openStore(t, scope)
	if err := s.PutMeta(localdb.Meta{DeviceID: "dev"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutKeyring([]byte("keyring-blob"), 0); err != nil {
		t.Fatal(err)
	}
	one, err := s.Append("ws", "c1", []byte("cipher1"), changes.HLC{WallMillis: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append("ws", "c2", []byte("cipher2"), changes.HLC{WallMillis: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkAcked(one, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.Compact("ws", []byte("snapshot"), one, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.PutDraft("route/form", []byte("draft")); err != nil {
		t.Fatal(err)
	}
	if err := s.Quarantine([]byte("raw"), "bad record"); err != nil {
		t.Fatal(err)
	}

	dump := dumpDB(t, localdb.DBName(scope))
	for _, store := range []string{"meta", "keyring", "log", "snapshot", "drafts", "quarantine"} {
		if len(dump[store]) == 0 {
			t.Errorf("object store %q has no records; dump=%v", store, dump)
		}
	}
	for _, k := range dumpKeys(dump["meta"]) {
		if k == "state" {
			t.Errorf("meta store still holds the legacy whole-state record; dump=%v", dump)
		}
	}
	// After compaction the acked entry is gone; the pending entry remains as
	// its own record keyed by its localSeq ("2").
	if keys := dumpKeys(dump["log"]); len(keys) != 1 || keys[0] != "2" {
		t.Errorf("log record keys = %v, want [2]", keys)
	}
	if keys := dumpKeys(dump["snapshot"]); len(keys) != 1 || keys[0] != "ws" {
		t.Errorf("snapshot record keys = %v, want [ws]", keys)
	}
	if keys := dumpKeys(dump["drafts"]); len(keys) != 1 || keys[0] != "route/form" {
		t.Errorf("draft record keys = %v, want [route/form]", keys)
	}
}

// legacyState mirrors the pre-SV3a.1 whole-state checkpoint format so the
// test can seed a database exactly as the old adapter wrote it.
type legacyState struct {
	Scope          keyring.Scope               `json:"scope"`
	Meta           localdb.Meta                `json:"meta"`
	Keyring        []byte                      `json:"keyring,omitempty"`
	KeyringVersion uint64                      `json:"keyringVersion"`
	Log            []localdb.LogEntry          `json:"log"`
	NextSeq        uint64                      `json:"nextSeq"`
	Snapshots      map[string]localdb.Snapshot `json:"snapshots"`
	Drafts         map[string][]byte           `json:"drafts"`
	Quarantine     []legacyQuarantine          `json:"quarantine"`
}
type legacyQuarantine struct {
	Raw    []byte
	Reason string
	At     time.Time
}

const seedLegacyJS = `
globalThis.__seedLegacyState = function (dbName, stateJSON) {
  return new Promise(function (resolve, reject) {
    var req = indexedDB.open(dbName, 1);
    req.onupgradeneeded = function () {
      var db = req.result;
      ["meta","keyring","log","snapshot","drafts","quarantine"].forEach(function (n) {
        if (!db.objectStoreNames.contains(n)) {
          var st = db.createObjectStore(n);
          if (n === "log") st.createIndex("changeId", "changeId", {unique: true});
        }
      });
    };
    req.onsuccess = function () {
      var db = req.result;
      var tx = db.transaction("meta", "readwrite");
      tx.objectStore("meta").put(stateJSON, "state");
      tx.oncomplete = function () { resolve(true); };
      tx.onerror = function () { reject(tx.error); };
      tx.onabort = function () { reject(tx.error); };
    };
    req.onerror = function () { reject(req.error); };
  });
};`

// TestIDBLegacyCheckpointMigrates: a database written by the old adapter
// (one meta/state blob) is converted to per-store records on Open, with no
// data lost and sequences continuing from where they left off.
func TestIDBLegacyCheckpointMigrates(t *testing.T) {
	fake := setupIDB(t)
	scope := keyring.LocalScope("legacy-acct")
	dbName := localdb.DBName(scope)
	deleteDB(t, dbName)

	state := legacyState{
		Scope:          scope,
		Meta:           localdb.Meta{DeviceID: "legacy-dev", Version: 4},
		Keyring:        []byte("legacy-keyring"),
		KeyringVersion: 2,
		Log: []localdb.LogEntry{
			{LocalSeq: 1, ChangeID: "old-1", WorkspaceID: "ws", Sealed: []byte("cipher-old-1"), ServerSeq: 9, CreatedHLC: changes.HLC{WallMillis: 1}},
			{LocalSeq: 2, ChangeID: "old-2", WorkspaceID: "ws", Sealed: []byte("cipher-old-2"), CreatedHLC: changes.HLC{WallMillis: 2}},
		},
		NextSeq:   2,
		Snapshots: map[string]localdb.Snapshot{"ws": {WorkspaceID: "ws", CoversLocalSeq: 1, CoversServerSeq: 9, Sealed: []byte("snap")}},
		Drafts:    map[string][]byte{"route/form": []byte("legacy-draft")},
		Quarantine: []legacyQuarantine{
			{Raw: []byte("junk"), Reason: "bad", At: time.Unix(1700000000, 0).UTC()},
		},
	}
	blob, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	js.Global().Call("eval", seedLegacyJS)
	awaitPromise(t, js.Global().Call("__seedLegacyState", dbName, string(blob)))

	s := openStore(t, scope)
	meta, err := s.Meta()
	if err != nil || meta.DeviceID != "legacy-dev" || meta.Version != 4 {
		t.Fatalf("meta = %+v err=%v", meta, err)
	}
	if blob2, err := s.Keyring(); err != nil || string(blob2) != "legacy-keyring" {
		t.Fatalf("keyring = %q err=%v", blob2, err)
	}
	// Keyring version survived: CAS against the wrong version still fails.
	if err := s.PutKeyring([]byte("new"), 0); !errors.Is(err, localdb.ErrConflict) {
		t.Fatalf("keyring CAS after migration err = %v, want ErrConflict", err)
	}
	entries, err := s.Since(0)
	if err != nil || len(entries) != 2 || entries[0].ChangeID != "old-1" || entries[1].LocalSeq != 2 {
		t.Fatalf("entries = %+v err=%v", entries, err)
	}
	if snap, err := s.LatestSnapshot("ws"); err != nil || string(snap.Sealed) != "snap" {
		t.Fatalf("snapshot = %+v err=%v", snap, err)
	}
	if draft, err := s.GetDraft("route/form"); err != nil || string(draft) != "legacy-draft" {
		t.Fatalf("draft = %q err=%v", draft, err)
	}
	seq, err := s.Append("ws", "new-3", []byte("cipher-new"), changes.HLC{WallMillis: 3})
	if err != nil {
		t.Fatal(err)
	}
	if seq != 3 {
		t.Fatalf("first post-migration seq = %d, want 3", seq)
	}
	if fake {
		dump := dumpDB(t, dbName)
		for _, k := range dumpKeys(dump["meta"]) {
			if k == "state" {
				t.Errorf("legacy meta/state record still present after migration")
			}
		}
		if keys := dumpKeys(dump["log"]); len(keys) != 3 {
			t.Errorf("log record keys after migration = %v, want 3 records", keys)
		}
		if len(dump["quarantine"]) != 1 {
			t.Errorf("quarantine records after migration = %d, want 1", len(dump["quarantine"]))
		}
	}
}

// TestIDBOpenFailureMappedAndRetryable: an open() rejection maps to the
// typed error for its DOMException name, the store stays closed (and bound
// to its scope), and a later Open can still succeed.
func TestIDBOpenFailureMappedAndRetryable(t *testing.T) {
	requireFake(t, setupIDB(t))
	scope := testScope()
	s := localdb.NewIDBStore()
	failNextOpen("SecurityError")
	err := s.Open(scope)
	if !errors.Is(err, localdb.ErrStorageSecurity) {
		t.Fatalf("open err = %v, want ErrStorageSecurity", err)
	}
	var storage *localdb.StorageError
	if !errors.As(err, &storage) || storage.Name != "SecurityError" {
		t.Fatalf("open err = %#v, want *StorageError named SecurityError", err)
	}
	if _, err := s.Meta(); !errors.Is(err, localdb.ErrInvalid) {
		t.Fatalf("Meta on unopened store err = %v, want ErrInvalid", err)
	}
	if err := s.Open(testScope()); !errors.Is(err, localdb.ErrConflict) {
		t.Fatalf("Open with a different scope err = %v, want ErrConflict", err)
	}
	if err := s.Open(scope); err != nil {
		t.Fatalf("retry Open: %v", err)
	}
	if _, err := s.Append("ws", "c1", []byte("cipher1"), changes.HLC{WallMillis: 1}); err != nil {
		t.Fatalf("append after retry: %v", err)
	}
}
