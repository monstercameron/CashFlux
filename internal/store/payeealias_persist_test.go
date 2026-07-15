// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestPayeeAliasCRUDAndRoundTrip(t *testing.T) {
	s := newStore(t)
	a := domain.PayeeAlias{
		ID:        "pa1",
		RawPayee:  "AMZN Mktp US*2K4RT0",
		Display:   "Amazon",
		CreatedAt: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
	}
	if err := s.PutPayeeAlias(a); err != nil {
		t.Fatalf("PutPayeeAlias: %v", err)
	}
	got, err := s.ListPayeeAliases()
	if err != nil {
		t.Fatalf("ListPayeeAliases: %v", err)
	}
	if len(got) != 1 || got[0].Display != "Amazon" || got[0].RawPayee != "AMZN Mktp US*2K4RT0" {
		t.Fatalf("aliases = %+v", got)
	}

	// Export/import lossless.
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.PayeeAliases) != 1 {
		t.Fatalf("snapshot aliases = %d, want 1", len(snap.PayeeAliases))
	}
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	g2, _ := s2.ListPayeeAliases()
	if len(g2) != 1 || g2[0].Display != "Amazon" || !g2[0].CreatedAt.Equal(a.CreatedAt) {
		t.Fatalf("round-trip mismatch: %+v", g2)
	}

	// Delete.
	ok, err := s.DeletePayeeAlias("pa1")
	if err != nil || !ok {
		t.Fatalf("DeletePayeeAlias = %v, %v", ok, err)
	}
	after, _ := s.ListPayeeAliases()
	if len(after) != 0 {
		t.Fatalf("after delete = %+v", after)
	}
}
