// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"errors"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

func jrnlBatch(t *testing.T, values, expected map[string]string) error {
	t.Helper()
	done := make(chan error, 1)
	SetSecureBatch(values, expected, func(err error) { done <- err })
	return waitResult(t, done)
}

func TestJRNLSnapshotAndReadOnlyCAS(t *testing.T) {
	initFixture(t)
	const checkpoint, head, generation = "cashflux:dataset", "cashflux:dataset:head", "cashflux:dataset:gen"
	const record = "cashflux:dataset:record:jrnl:one"
	if err := jrnlBatch(t, map[string]string{checkpoint: "checkpoint", head: "head-1", generation: "g1", record: "row-1"}, nil); err != nil {
		t.Fatal(err)
	}
	before, _, _ := (idbRecords{}).Read(checkpoint)
	snapshot, err := ReadSecureSnapshot([]string{checkpoint, head, generation}, []string{"cashflux:dataset:record:jrnl:"})
	if err != nil {
		t.Fatal(err)
	}
	if err := jrnlBatch(t, map[string]string{head: "head-2", generation: "g2", record: "row-2"}, map[string]string{checkpoint: "checkpoint", head: "head-1", generation: "g1"}); err != nil {
		t.Fatal(err)
	}
	after, _, _ := (idbRecords{}).Read(checkpoint)
	if before != after {
		t.Fatal("read-only checkpoint condition rewrote the full checkpoint")
	}
	if snapshot.Values[head] != "head-1" || snapshot.Values[generation] != "g1" || snapshot.Values[record] != "row-1" {
		t.Fatal("snapshot changed underneath its reader")
	}
	current, err := ReadSecureSnapshot([]string{checkpoint, head, generation}, []string{"cashflux:dataset:record:jrnl:"})
	if err != nil || current.Values[head] != "head-2" || current.Values[generation] != "g2" || current.Values[record] != "row-2" {
		t.Fatalf("split snapshot: %+v %v", current, err)
	}
	if err := jrnlBatch(t, map[string]string{head: "bad-head", generation: "bad-gen"}, map[string]string{checkpoint: "checkpoint", head: "head-1", generation: "g1"}); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("stale append = %v", err)
	}
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	if err := jrnlBatch(t, map[string]string{checkpoint: "compacted", head: "", generation: "g3", record: ""}, map[string]string{generation: "g2", head: "head-2", checkpoint: "checkpoint"}); err == nil {
		t.Fatal("aborted compaction acknowledged")
	}
	current, err = ReadSecureSnapshot([]string{checkpoint, head, generation, record}, nil)
	if err != nil || current.Values[checkpoint] != "checkpoint" || current.Values[head] != "head-2" || current.Values[generation] != "g2" || current.Values[record] != "row-2" {
		t.Fatal("aborted batch published partial checkpoint/head/generation/record")
	}
}

func TestJRNLSnapshotRecoveryCASIncludesUnreadableCiphertext(t *testing.T) {
	initFixture(t)
	const key = "cashflux:dataset"
	if err := (idbRecords{}).Replace(key, "damaged encrypted checkpoint", nil); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadSecureSnapshot([]string{key}, nil)
	if err != nil || snapshot.Errors[key] == nil {
		t.Fatalf("unreadable record hidden: %v %+v", err, snapshot)
	}
	if raw, have := snapshot.EncodedRecord(key); !have || raw != "damaged encrypted checkpoint" {
		t.Fatal("original unreadable bytes lost")
	}
	done := make(chan error, 1)
	SetSecureSnapshotBatch(map[string]string{key: "verified backup"}, snapshot, []string{key}, func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	SetSecureSnapshotBatch(map[string]string{key: "obsolete repair"}, snapshot, []string{key}, func(err error) { done <- err })
	if err := waitResult(t, done); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("stale repair = %v", err)
	}
	value, _, err := GetSecure(key)
	if err != nil || value != "verified backup" {
		t.Fatalf("recovery overwritten: %q %v", value, err)
	}
	LockSecure()
	if _, err := ReadSecureSnapshot([]string{key}, nil); !errors.Is(err, atrest.ErrLocked) {
		t.Fatalf("locked snapshot = %v", err)
	}
}
