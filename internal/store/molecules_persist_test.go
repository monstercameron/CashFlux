// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestMoleculeCRUDAndRoundTrip(t *testing.T) {
	s := newStore(t)
	m := domain.Molecule{Name: "net_worth", Formula: "assets - liabilities - goal_needs", Doc: "after goal set-asides"}
	if err := s.PutMolecule(m); err != nil {
		t.Fatalf("PutMolecule: %v", err)
	}
	got, err := s.ListMolecules()
	if err != nil {
		t.Fatalf("ListMolecules: %v", err)
	}
	if len(got) != 1 || got[0].Formula != m.Formula {
		t.Fatalf("molecules = %+v", got)
	}

	// Export/import lossless.
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Molecules) != 1 {
		t.Fatalf("snapshot molecules = %d, want 1", len(snap.Molecules))
	}
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	g2, _ := s2.ListMolecules()
	if len(g2) != 1 || g2[0].Name != "net_worth" || g2[0].Doc != "after goal set-asides" {
		t.Fatalf("round-trip mismatch: %+v", g2)
	}

	if ok, err := s.DeleteMolecule("net_worth"); err != nil || !ok {
		t.Fatalf("DeleteMolecule: ok=%v err=%v", ok, err)
	}
	if g3, _ := s.ListMolecules(); len(g3) != 0 {
		t.Fatalf("molecule not deleted: %+v", g3)
	}
}
