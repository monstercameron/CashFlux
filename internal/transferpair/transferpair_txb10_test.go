// SPDX-License-Identifier: MIT

package transferpair

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestDeclaredHonoursTransferGroupIdentity(t *testing.T) {
	cases := []struct {
		name      string
		legGroup  string
		group     string
		wantFound bool
	}{
		{name: "counterpart missing group", legGroup: "group-a", group: "", wantFound: false},
		{name: "leg missing group", legGroup: "", group: "group-a", wantFound: false},
		{name: "different groups", legGroup: "group-a", group: "group-b", wantFound: false},
		{name: "same group", legGroup: "group-a", group: "group-a", wantFound: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leg := asTransfer(tx("out", "chk", "Transfer", -50000, day(14)), "sav")
			leg.TransferGroupID = tc.legGroup
			candidate := asTransfer(tx("in", "sav", "Transfer", 50000, day(14)), "chk")
			candidate.TransferGroupID = tc.group
			got, ok := Declared(leg, []domain.Transaction{leg, candidate})
			if ok != tc.wantFound {
				t.Fatalf("Declared found=%v, want %v (partner %q)", ok, tc.wantFound, got.ID)
			}
			if ok && got.ID != candidate.ID {
				t.Errorf("partner = %q, want %q", got.ID, candidate.ID)
			}
		})
	}
}

func TestDeclaredKeepsLegacyUngroupedPairs(t *testing.T) {
	leg := asTransfer(tx("out", "chk", "Transfer", -50000, day(14)), "sav")
	candidate := asTransfer(tx("in", "sav", "Transfer", 50000, day(14)), "chk")

	got, ok := Declared(leg, []domain.Transaction{leg, candidate})
	if !ok || got.ID != candidate.ID {
		t.Fatalf("Declared = (%q, %v), want legacy ungrouped partner %q", got.ID, ok, candidate.ID)
	}
}

func TestGroupedLegDoesNotHeuristicallyAdoptUngroupedLookalike(t *testing.T) {
	leg := asTransfer(tx("out", "chk", "Transfer to Savings *6500", -50000, day(14)), "sav")
	leg.TransferGroupID = "group-a"
	lookalike := tx("decoy", "sav", "Transfer from Checking *8945", 50000, day(14))

	match := For(leg, []domain.Transaction{leg, lookalike}, accounts())
	if match.Confidence != Unmatched {
		t.Fatalf("For matched explicit group to look-alike: confidence=%s partner=%q", match.Confidence, match.Partner.ID)
	}
}
