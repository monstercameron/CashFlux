// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSQLiteRoundTrip(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer s.Close()

	ds := sampleDataset()
	if err := s.Load(ds); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	want, _ := Export(ds)
	have, _ := Export(got)
	if !bytes.Equal(want, have) {
		t.Errorf("SQLite round trip not lossless:\nwant:\n%s\nhave:\n%s", want, have)
	}
}

func TestSQLiteReloadReplaces(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer s.Close()

	if err := s.Load(sampleDataset()); err != nil {
		t.Fatalf("Load 1: %v", err)
	}
	// Loading again should fully replace the previous contents.
	if err := s.Load(Dataset{Members: []domain.Member{{ID: "x", Name: "Solo"}}}); err != nil {
		t.Fatalf("Load 2: %v", err)
	}
	got, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(got.Members) != 1 || got.Members[0].Name != "Solo" {
		t.Errorf("members not replaced: %+v", got.Members)
	}
	if len(got.Accounts) != 0 || len(got.Transactions) != 0 {
		t.Errorf("old rows not cleared: %d accounts, %d txns", len(got.Accounts), len(got.Transactions))
	}
}

func TestSQLiteEmptySnapshot(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer s.Close()

	got, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("schema version = %d, want %d", got.SchemaVersion, SchemaVersion)
	}
	if len(got.Members) != 0 {
		t.Errorf("expected empty store, got %d members", len(got.Members))
	}
}
