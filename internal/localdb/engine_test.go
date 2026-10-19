// SPDX-License-Identifier: MIT
package localdb

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/changes"
	"github.com/monstercameron/CashFlux/internal/keyring"
)

func fixture(t *testing.T) (*Engine, *keyring.WorkspaceKey, *MemStore) {
	t.Helper()
	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	e, err := NewEngine(store, keyring.LocalScope("acct"), "ws")
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Boot(wk); err != nil {
		t.Fatal(err)
	}
	return e, wk, store
}

func makeBatch(t *testing.T, id string, logical uint32) changes.Batch {
	t.Helper()
	cid, err := changes.NewChangeID()
	if err != nil {
		t.Fatal(err)
	}
	return changes.Batch{ChangeID: cid, Scope: changes.Scope{ServerID: keyring.LocalServerID, AccountID: "acct"}, WorkspaceID: "ws", DeviceID: "dev", HLC: changes.HLC{WallMillis: 1, Logical: logical}, SchemaVersion: changes.CurrentSchemaVersion, Ops: []changes.Op{{Table: "things", RecordID: id, Fields: map[string]json.RawMessage{"value": json.RawMessage(`"` + id + `"`)}}}}
}

func TestEngineAppendReplayAndCrashRecovery(t *testing.T) {
	e, wk, store := fixture(t)
	b := makeBatch(t, "one", 1)
	if err := e.Commit(b); err != nil {
		t.Fatal(err)
	}
	restarted, _ := NewEngine(store, keyring.LocalScope("acct"), "ws")
	if err := restarted.Boot(wk); err != nil {
		t.Fatal(err)
	}
	s, _ := restarted.State()
	if got, ok := s.Materialize("things", "one"); !ok || string(got) != `{"value":"one"}` {
		t.Fatalf("replayed = %s, %v", got, ok)
	}
}

func TestEngineAADMismatchQuarantines(t *testing.T) {
	e, wk, store := fixture(t)
	b := makeBatch(t, "one", 1)
	if err := e.Commit(b); err != nil {
		t.Fatal(err)
	}
	store.log[0].Sealed[0] ^= 1
	restarted, _ := NewEngine(store, keyring.LocalScope("acct"), "ws")
	if err := restarted.Boot(wk); err == nil {
		t.Fatal("expected authentication error")
	}
	if len(store.quarantine) != 1 {
		t.Fatalf("quarantine count=%d", len(store.quarantine))
	}
}

func TestEngineCompactionRetainsPendingAndReplays(t *testing.T) {
	e, wk, store := fixture(t)
	first := makeBatch(t, "first", 1)
	if err := e.Commit(first); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAcked(1, 4); err != nil {
		t.Fatal(err)
	}
	pending := makeBatch(t, "pending", 2)
	if err := e.Commit(pending); err != nil {
		t.Fatal(err)
	}
	if err := e.Compact(); err != nil {
		t.Fatal(err)
	}
	entries, err := store.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ChangeID != pending.ChangeID || entries[0].ServerSeq != 0 {
		t.Fatalf("pending entries after compact = %+v", entries)
	}
	restarted, _ := NewEngine(store, keyring.LocalScope("acct"), "ws")
	if err := restarted.Boot(wk); err != nil {
		t.Fatal(err)
	}
	state, _ := restarted.State()
	for _, id := range []string{"first", "pending"} {
		if !state.Exists("things", id) {
			t.Errorf("record %q missing after compact/replay", id)
		}
	}
}
