// SPDX-License-Identifier: MIT

// Package storetest contains the shared Store contract suite.
package storetest

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/changes"
	"github.com/monstercameron/CashFlux/internal/keyring"
	"github.com/monstercameron/CashFlux/internal/localdb"
	"testing"
)

// Factory constructs a fresh Store for one conformance run.
type Factory func() localdb.Store

// Conformance checks the behavior required of every Store implementation.
func Conformance(t *testing.T, makeStore Factory) {
	t.Helper()
	s := makeStore()
	if err := s.Open(keyring.LocalScope("acct")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Meta(); err != nil {
		t.Fatalf("empty metadata: %v", err)
	}
	if err := s.PutMeta(localdb.Meta{DeviceID: "dev"}); err != nil {
		t.Fatal(err)
	}
	meta, err := s.Meta()
	if err != nil || meta.DeviceID != "dev" || meta.Version == 0 || meta.Scope != keyring.LocalScope("acct") {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
	if err = s.PutKeyring([]byte("keyring"), 0); err != nil {
		t.Fatal(err)
	}
	if err = s.PutKeyring([]byte("replacement"), 0); !errors.Is(err, localdb.ErrConflict) {
		t.Fatalf("expected keyring CAS conflict, got %v", err)
	}
	one, err := s.Append("ws", "c1", []byte("cipher1"), changes.HLC{WallMillis: 1})
	if err != nil {
		t.Fatal(err)
	}
	two, err := s.Append("ws", "c2", []byte("cipher2"), changes.HLC{WallMillis: 2})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.Pending(1)
	if err != nil || len(pending) != 1 || pending[0].LocalSeq != one {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	if err = s.MarkAcked(one, 11); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AppendRemote("ws", "remote", []byte("cipher3"), 12, changes.HLC{WallMillis: 3}); err != nil {
		t.Fatal(err)
	}
	seq, err := s.AppendRemote("ws", "remote", []byte("cipher3"), 12, changes.HLC{WallMillis: 3})
	if err != nil || seq == 0 {
		t.Fatalf("remote dedupe seq=%d err=%v", seq, err)
	}
	if _, err = s.AppendRemote("other-workspace", "remote", []byte("cipher3"), 12, changes.HLC{WallMillis: 3}); !errors.Is(err, localdb.ErrConflict) {
		t.Fatalf("expected conflicting remote duplicate to fail, got %v", err)
	}
	if err = s.Compact("ws", []byte("snapshot"), two, 11); err != nil {
		t.Fatal(err)
	}
	if _, err = s.LatestSnapshot("ws"); err != nil {
		t.Fatal(err)
	}
	entries, err := s.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].LocalSeq != two || entries[1].ServerSeq != 12 {
		t.Fatalf("compaction entries=%+v", entries)
	}
	if err = s.PutDraft("route/form", []byte("draft")); err != nil {
		t.Fatal(err)
	}
	draft, err := s.GetDraft("route/form")
	if err != nil || string(draft) != "draft" {
		t.Fatalf("draft=%q err=%v", draft, err)
	}
	if err = s.DeleteDraft("route/form"); err != nil {
		t.Fatal(err)
	}
	if err = s.Quarantine([]byte("raw"), "bad"); err != nil {
		t.Fatal(err)
	}
	if quarantined, err := s.IsQuarantined([]byte("raw")); err != nil || !quarantined {
		t.Fatalf("IsQuarantined = %v, err = %v", quarantined, err)
	}
}
