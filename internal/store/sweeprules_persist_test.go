// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSweepRuleRoundTrip(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	r := domain.SweepRule{
		ID: "s1", SourceAccountID: "chk", DestAccountID: "sav",
		KeepMinor: 3000_00, Cadence: domain.SweepMonthly, Enabled: true,
	}
	if err := s.PutSweepRule(r); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetSweepRule("s1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got != r {
		t.Errorf("round-trip mismatch: %+v != %+v", got, r)
	}
	list, err := s.ListSweepRules()
	if err != nil || len(list) != 1 {
		t.Fatalf("list len=%d err=%v", len(list), err)
	}

	// Snapshot/load round-trip preserves the rule.
	ds, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.SweepRules) != 1 {
		t.Fatalf("snapshot sweep rules = %d, want 1", len(ds.SweepRules))
	}
	s2, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Load(ds); err != nil {
		t.Fatal(err)
	}
	list2, _ := s2.ListSweepRules()
	if len(list2) != 1 || list2[0].KeepMinor != 3000_00 {
		t.Errorf("import round-trip failed: %+v", list2)
	}

	// Delete.
	if _, err := s.DeleteSweepRule("s1"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.GetSweepRule("s1"); ok {
		t.Error("expected rule to be deleted")
	}
}
