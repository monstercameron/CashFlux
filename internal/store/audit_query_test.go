// SPDX-License-Identifier: MIT

package store_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestListAuditEntriesByEntityFiltersOrdersAndLimits(t *testing.T) {
	t.Parallel()
	st, err := store.NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	base := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	entries := []auditlog.Entry{
		{ID: "z-old", At: base, EntityType: "transaction", EntityID: "tx-1"},
		{ID: "a-new", At: base.Add(2 * time.Minute), EntityType: "transaction", EntityID: "tx-1"},
		{ID: "m-middle", At: base.Add(time.Minute), EntityType: "transaction", EntityID: "tx-1"},
		{ID: "other-type", At: base.Add(3 * time.Minute), EntityType: "account", EntityID: "tx-1"},
		{ID: "other-id", At: base.Add(4 * time.Minute), EntityType: "transaction", EntityID: "tx-2"},
	}
	for _, entry := range entries {
		if err := st.PutAuditEntry(entry); err != nil {
			t.Fatalf("PutAuditEntry(%q): %v", entry.ID, err)
		}
	}

	tests := []struct {
		name  string
		limit int
		want  []string
	}{
		{name: "all", want: []string{"a-new", "m-middle", "z-old"}},
		{name: "limited", limit: 2, want: []string{"a-new", "m-middle"}},
		{name: "non-positive limit", limit: -1, want: []string{"a-new", "m-middle", "z-old"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := st.ListAuditEntriesByEntity("transaction", "tx-1", tt.limit)
			if err != nil {
				t.Fatalf("ListAuditEntriesByEntity: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for i, wantID := range tt.want {
				if got[i].ID != wantID {
					t.Errorf("entry %d id = %q, want %q", i, got[i].ID, wantID)
				}
			}
		})
	}
}

func TestPutTasksAtomicRollsBackOnDuplicateID(t *testing.T) {
	t.Parallel()
	st, err := store.NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	original := domain.Task{ID: "task-1", Title: "original", Status: domain.StatusOpen}
	if err := st.PutTask(original); err != nil {
		t.Fatalf("PutTask: %v", err)
	}
	done := original
	done.Status = domain.StatusDone
	successor := domain.Task{ID: original.ID, Title: "successor", Status: domain.StatusOpen}
	if err := st.PutTasksAtomic(done, successor); err == nil {
		t.Fatal("PutTasksAtomic succeeded with duplicate IDs, want error")
	}
	got, ok, err := st.GetTask(original.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if !ok || got.Status != original.Status || got.Title != original.Title {
		t.Fatalf("task after failed atomic write = %+v (found %v), want original %+v", got, ok, original)
	}
}
