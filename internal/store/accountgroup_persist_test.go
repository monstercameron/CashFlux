// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestAccountGroupCRUDAndRoundTrip(t *testing.T) {
	s := newStore(t)
	g := domain.AccountGroup{
		ID:         "grp1",
		Name:       "Liquid",
		AccountIDs: []string{"chk", "sav"},
		Order:      1,
		VarName:    "liquid",
	}
	if err := s.PutAccountGroup(g); err != nil {
		t.Fatalf("PutAccountGroup: %v", err)
	}
	got, err := s.ListAccountGroups()
	if err != nil {
		t.Fatalf("ListAccountGroups: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Liquid" || len(got[0].AccountIDs) != 2 {
		t.Fatalf("groups = %+v", got)
	}

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.AccountGroups) != 1 {
		t.Fatalf("snapshot groups = %d, want 1", len(snap.AccountGroups))
	}
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	g2, _ := s2.ListAccountGroups()
	if len(g2) != 1 || g2[0].VarName != "liquid" || g2[0].AccountIDs[1] != "sav" {
		t.Fatalf("round-trip mismatch: %+v", g2)
	}

	if ok, err := s.DeleteAccountGroup("grp1"); err != nil || !ok {
		t.Fatalf("DeleteAccountGroup: ok=%v err=%v", ok, err)
	}
	if g3, _ := s.ListAccountGroups(); len(g3) != 0 {
		t.Fatalf("group not deleted: %+v", g3)
	}
}
