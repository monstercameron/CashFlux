// SPDX-License-Identifier: MIT

package store_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/store"
)

// TestAuditLogRoundTrip verifies that audit entries written via PutAuditEntry are
// returned by ListAuditEntries in reverse-chronological order (newest first), and
// that a full Load→Snapshot round-trip through the Dataset is lossless.
func TestAuditLogRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	entries := []auditlog.Entry{
		{ID: "ae-1", At: now.Add(-2 * time.Minute), Actor: "user", Action: "added", EntityType: "transaction", EntityID: "tx-1", Summary: "Added transaction tx-1"},
		{ID: "ae-2", At: now.Add(-1 * time.Minute), Actor: "user", Action: "updated", EntityType: "account", EntityID: "ac-1", Summary: "Updated account ac-1"},
		{ID: "ae-3", At: now, Actor: "user", Action: "deleted", EntityType: "budget", EntityID: "bg-1", Summary: "Deleted budget bg-1"},
	}

	t.Run("PutAndList", func(t *testing.T) {
		st, err := store.NewMemory()
		if err != nil {
			t.Fatalf("NewMemory: %v", err)
		}
		defer st.Close()

		for _, e := range entries {
			if err := st.PutAuditEntry(e); err != nil {
				t.Fatalf("PutAuditEntry %s: %v", e.ID, err)
			}
		}

		got, err := st.ListAuditEntries(0)
		if err != nil {
			t.Fatalf("ListAuditEntries: %v", err)
		}
		if len(got) != len(entries) {
			t.Fatalf("want %d entries, got %d", len(entries), len(got))
		}
		// ListAuditEntries returns newest-first (reversed from insertion order).
		if got[0].ID != "ae-3" || got[1].ID != "ae-2" || got[2].ID != "ae-1" {
			t.Fatalf("unexpected order: got %v %v %v", got[0].ID, got[1].ID, got[2].ID)
		}
	})

	t.Run("ListWithLimit", func(t *testing.T) {
		st, err := store.NewMemory()
		if err != nil {
			t.Fatalf("NewMemory: %v", err)
		}
		defer st.Close()

		for _, e := range entries {
			if err := st.PutAuditEntry(e); err != nil {
				t.Fatalf("PutAuditEntry %s: %v", e.ID, err)
			}
		}

		got, err := st.ListAuditEntries(2)
		if err != nil {
			t.Fatalf("ListAuditEntries(2): %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 entries, got %d", len(got))
		}
		// Limit 2 newest: ae-3 then ae-2.
		if got[0].ID != "ae-3" || got[1].ID != "ae-2" {
			t.Fatalf("unexpected limited order: got %v %v", got[0].ID, got[1].ID)
		}
	})

	t.Run("DatasetRoundTrip", func(t *testing.T) {
		st, err := store.NewMemory()
		if err != nil {
			t.Fatalf("NewMemory: %v", err)
		}
		defer st.Close()

		ds := store.EmptyDataset()
		ds.AuditEntries = entries

		if err := st.Load(ds); err != nil {
			t.Fatalf("Load: %v", err)
		}

		snap, err := st.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if len(snap.AuditEntries) != len(entries) {
			t.Fatalf("round-trip: want %d entries, got %d", len(entries), len(snap.AuditEntries))
		}
		// Snapshot loads in id-order (oldest-first by id string ordering).
		byID := map[string]auditlog.Entry{}
		for _, e := range snap.AuditEntries {
			byID[e.ID] = e
		}
		for _, orig := range entries {
			got, ok := byID[orig.ID]
			if !ok {
				t.Errorf("missing entry %s after round-trip", orig.ID)
				continue
			}
			if got.Summary != orig.Summary {
				t.Errorf("entry %s: summary mismatch: got %q want %q", orig.ID, got.Summary, orig.Summary)
			}
		}
	})

	t.Run("CapDrop", func(t *testing.T) {
		// Build a dataset with AuditLogCap+5 entries; after Load→Snapshot only
		// AuditLogCap entries should survive (oldest are dropped).
		total := store.AuditLogCap + 5
		big := make([]auditlog.Entry, total)
		for i := range big {
			big[i] = auditlog.Entry{
				ID:      "ae-cap-" + padInt(i),
				Actor:   "user",
				Action:  "added",
				Summary: "entry",
			}
		}

		st, err := store.NewMemory()
		if err != nil {
			t.Fatalf("NewMemory: %v", err)
		}
		defer st.Close()

		ds := store.EmptyDataset()
		ds.AuditEntries = big
		if err := st.Load(ds); err != nil {
			t.Fatalf("Load: %v", err)
		}

		snap, err := st.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if len(snap.AuditEntries) != store.AuditLogCap {
			t.Fatalf("cap: want %d entries, got %d", store.AuditLogCap, len(snap.AuditEntries))
		}
		// The 5 oldest entries should have been dropped.
		if snap.AuditEntries[0].ID != "ae-cap-"+padInt(5) {
			t.Fatalf("cap: oldest surviving entry should be index 5, got %s", snap.AuditEntries[0].ID)
		}
	})
}

// padInt formats i as a zero-padded 4-digit string for deterministic id ordering.
func padInt(i int) string {
	s := "0000" + intToStr(i)
	return s[len(s)-4:]
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
