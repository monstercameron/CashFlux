// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTxnLinkCRUDAndRoundTrip(t *testing.T) {
	s := newStore(t)
	l := domain.TxnLink{
		ID:        "link1",
		Kind:      domain.TxnLinkRefundPair,
		TxnIDs:    []string{"orig", "refund"},
		Amount:    money.New(4000, "USD"),
		Note:      "partial refund of jacket",
		CreatedAt: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
	}
	if err := s.PutTxnLink(l); err != nil {
		t.Fatalf("PutTxnLink: %v", err)
	}
	got, err := s.ListTxnLinks()
	if err != nil {
		t.Fatalf("ListTxnLinks: %v", err)
	}
	if len(got) != 1 || got[0].Kind != domain.TxnLinkRefundPair || got[0].Amount.Amount != 4000 {
		t.Fatalf("txnlinks = %+v", got)
	}

	// Export/import lossless.
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.TxnLinks) != 1 {
		t.Fatalf("snapshot txnlinks = %d, want 1", len(snap.TxnLinks))
	}
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	g2, _ := s2.ListTxnLinks()
	if len(g2) != 1 || len(g2[0].TxnIDs) != 2 || g2[0].TxnIDs[0] != "orig" || g2[0].Note != "partial refund of jacket" {
		t.Fatalf("round-trip mismatch: %+v", g2)
	}

	if ok, err := s.DeleteTxnLink("link1"); err != nil || !ok {
		t.Fatalf("DeleteTxnLink: ok=%v err=%v", ok, err)
	}
	if g3, _ := s.ListTxnLinks(); len(g3) != 0 {
		t.Fatalf("txnlink not deleted: %+v", g3)
	}
}
