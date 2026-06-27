// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestBalanceSnapshotCRUD(t *testing.T) {
	s := newStore(t)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	snap := domain.BalanceSnapshot{
		ID:           "snap1",
		AccountID:    "acct1",
		BalanceMinor: 250000,
		Currency:     "USD",
		AsOf:         now,
	}

	if err := s.PutBalanceSnapshot(snap); err != nil {
		t.Fatalf("PutBalanceSnapshot: %v", err)
	}

	list, err := s.ListBalanceSnapshots("acct1")
	if err != nil {
		t.Fatalf("ListBalanceSnapshots: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(list))
	}
	got := list[0]
	if got.ID != "snap1" || got.AccountID != "acct1" || got.BalanceMinor != 250000 ||
		got.Currency != "USD" || !got.AsOf.Equal(now) {
		t.Errorf("snapshot fields wrong: %+v", got)
	}

	// A second snapshot for the same account.
	snap2 := domain.BalanceSnapshot{
		ID:           "snap2",
		AccountID:    "acct1",
		BalanceMinor: 260000,
		Currency:     "USD",
		AsOf:         now.Add(24 * time.Hour),
	}
	if err := s.PutBalanceSnapshot(snap2); err != nil {
		t.Fatalf("PutBalanceSnapshot snap2: %v", err)
	}

	list2, err := s.ListBalanceSnapshots("acct1")
	if err != nil {
		t.Fatalf("ListBalanceSnapshots after second: %v", err)
	}
	if len(list2) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(list2))
	}

	// Different account should return nothing.
	other, err := s.ListBalanceSnapshots("other-account")
	if err != nil {
		t.Fatalf("ListBalanceSnapshots other: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("expected 0 snapshots for other account, got %d", len(other))
	}
}

func TestDatasetBalanceSnapshotRoundTrip(t *testing.T) {
	asOf := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	ds := sampleDataset()
	ds.BalanceSnapshots = []domain.BalanceSnapshot{
		{
			ID:           "bs1",
			AccountID:    "acc-a",
			BalanceMinor: 500000,
			Currency:     "USD",
			AsOf:         asOf,
		},
		{
			ID:           "bs2",
			AccountID:    "acc-a",
			BalanceMinor: 520000,
			Currency:     "USD",
			AsOf:         asOf.Add(30 * 24 * time.Hour),
		},
	}

	first, err := Export(ds)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	imported, err := Import(first)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	second, err := Export(imported)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("balance snapshot round-trip not lossless")
	}

	if len(imported.BalanceSnapshots) != 2 {
		t.Fatalf("snapshots lost: got %d, want 2", len(imported.BalanceSnapshots))
	}
	if imported.BalanceSnapshots[0].ID != "bs1" || imported.BalanceSnapshots[0].BalanceMinor != 500000 {
		t.Errorf("snapshot[0] mismatch: %+v", imported.BalanceSnapshots[0])
	}
	if imported.BalanceSnapshots[1].ID != "bs2" || imported.BalanceSnapshots[1].BalanceMinor != 520000 {
		t.Errorf("snapshot[1] mismatch: %+v", imported.BalanceSnapshots[1])
	}
}
