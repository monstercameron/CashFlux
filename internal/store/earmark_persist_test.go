// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEarmarkRoundTrip(t *testing.T) {
	asOf := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	ds := Dataset{
		Earmarks: []domain.Earmark{
			{
				ID:              "em1",
				DestinationID:   "acc-savings",
				DestinationKind: domain.EarmarkKindAccount,
				Amount:          money.New(50000, "USD"),
				Currency:        "USD",
				CreatedAt:       asOf,
				Note:            "emergency fund top-up",
			},
			{
				ID:              "em2",
				DestinationID:   "acc-visa",
				DestinationKind: domain.EarmarkKindDebt,
				Amount:          money.New(20000, "USD"),
				Currency:        "USD",
				CreatedAt:       asOf,
			},
		},
	}

	exported, err := Export(ds)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	imported, err := Import(exported)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	reexported, err := Export(imported)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if !bytes.Equal(exported, reexported) {
		t.Errorf("round-trip not lossless:\nfirst:\n%s\nsecond:\n%s", exported, reexported)
	}
	if len(imported.Earmarks) != 2 {
		t.Fatalf("earmarks count = %d, want 2", len(imported.Earmarks))
	}
	em := imported.Earmarks[0]
	if em.ID != "em1" {
		t.Errorf("ID = %q, want %q", em.ID, "em1")
	}
	if em.DestinationKind != domain.EarmarkKindAccount {
		t.Errorf("DestinationKind = %q, want %q", em.DestinationKind, domain.EarmarkKindAccount)
	}
	if em.Amount.Amount != 50000 || em.Amount.Currency != "USD" {
		t.Errorf("Amount = %+v, want {50000 USD}", em.Amount)
	}
	if em.Note != "emergency fund top-up" {
		t.Errorf("Note = %q, want %q", em.Note, "emergency fund top-up")
	}
	if !em.CreatedAt.Equal(asOf) {
		t.Errorf("CreatedAt = %v, want %v", em.CreatedAt, asOf)
	}

	// Verify the SQLite load/snapshot path also round-trips.
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()
	if err := st.Load(imported); err != nil {
		t.Fatalf("load: %v", err)
	}
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Earmarks) != 2 {
		t.Fatalf("snapshot earmarks count = %d, want 2", len(snap.Earmarks))
	}
	if snap.Earmarks[1].DestinationKind != domain.EarmarkKindDebt {
		t.Errorf("snapshot em2 kind = %q, want debt", snap.Earmarks[1].DestinationKind)
	}
}

func TestEarmarkCRUD(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()

	em := domain.Earmark{
		ID:              "x1",
		DestinationID:   "some-account",
		DestinationKind: domain.EarmarkKindAccount,
		Amount:          money.New(10000, "USD"),
		Currency:        "USD",
		CreatedAt:       time.Now().UTC().Truncate(time.Second),
	}
	if err := st.PutEarmark(em); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok, err := st.GetEarmark("x1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Amount.Amount != 10000 {
		t.Errorf("amount = %d, want 10000", got.Amount.Amount)
	}
	list, err := st.ListEarmarks()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	deleted, err := st.DeleteEarmark("x1")
	if err != nil || !deleted {
		t.Fatalf("delete: ok=%v err=%v", deleted, err)
	}
	list2, _ := st.ListEarmarks()
	if len(list2) != 0 {
		t.Errorf("after delete: len=%d, want 0", len(list2))
	}
}
