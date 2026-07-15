// SPDX-License-Identifier: MIT

package txnlinks

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func txn(id, payee string, amt int64, d time.Time) domain.Transaction {
	return domain.Transaction{ID: id, Payee: payee, Amount: usd(amt), Date: d}
}

func TestGroupOfAndMembers(t *testing.T) {
	links := []domain.TxnLink{
		{ID: "g1", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b", "c"}},
		{ID: "p1", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"x", "y"}},
	}
	tests := []struct {
		txnID   string
		wantOK  bool
		wantGID string
	}{
		{"a", true, "g1"},
		{"c", true, "g1"},
		{"x", false, ""}, // refund-pair member is not an order-group member
		{"z", false, ""},
	}
	for _, tc := range tests {
		got, ok := GroupOf(tc.txnID, links)
		if ok != tc.wantOK || got.ID != tc.wantGID {
			t.Errorf("GroupOf(%q) = %q,%v want %q,%v", tc.txnID, got.ID, ok, tc.wantGID, tc.wantOK)
		}
	}
}

func TestGroupSumAndReconcile(t *testing.T) {
	members := []domain.Transaction{
		txn("a", "Store", -4000, day(2026, 3, 1)),
		txn("b", "Store", -3500, day(2026, 3, 2)),
		txn("c", "Store", -2500, day(2026, 3, 3)),
	}
	sum := GroupSum(members)
	if sum.Amount != -10000 {
		t.Fatalf("GroupSum = %d, want -10000", sum.Amount)
	}
	tests := []struct {
		name          string
		entered       money.Money
		wantRemainder int64
		wantBalanced  bool
	}{
		{"balanced", usd(10000), 0, true},
		{"under", usd(12000), 2000, false}, // members don't yet cover order
		{"over", usd(9000), -1000, false},  // members overshoot order
		{"none-entered", money.Money{}, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rem, bal := Reconcile(sum, tc.entered)
			if rem.Amount != tc.wantRemainder || bal != tc.wantBalanced {
				t.Errorf("Reconcile = %d,%v want %d,%v", rem.Amount, bal, tc.wantRemainder, tc.wantBalanced)
			}
		})
	}
}

func TestRefundNet(t *testing.T) {
	original := txn("o", "Jacket Co", -12000, day(2026, 3, 10))
	tests := []struct {
		name    string
		linkAmt money.Money
		refund  domain.Transaction
		wantNet int64
	}{
		{"full", money.Money{}, txn("r", "Jacket Co", 12000, day(2026, 4, 5)), 12000},
		{"partial-explicit", usd(4000), txn("r", "Jacket Co", 4000, day(2026, 4, 5)), 4000},
		{"partial-from-refund-amt", money.Money{}, txn("r", "Jacket Co", 4000, day(2026, 4, 5)), 4000},
		{"capped-at-original", usd(20000), txn("r", "Jacket Co", 20000, day(2026, 4, 5)), 12000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"o", "r"}, Amount: tc.linkAmt}
			got := RefundNet(l, original, tc.refund)
			if got.Amount != tc.wantNet {
				t.Errorf("RefundNet = %d, want %d", got.Amount, tc.wantNet)
			}
		})
	}
}

func TestNetAdjustmentsAndTransactions(t *testing.T) {
	original := txn("o", "Jacket Co", -12000, day(2026, 3, 10))
	refund := txn("r", "Jacket Co", 4000, day(2026, 4, 5))
	txns := []domain.Transaction{original, refund}
	links := []domain.TxnLink{
		{ID: "p", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"o", "r"}, Amount: usd(4000)},
	}

	adj := NetAdjustments(txns, links)
	if adj["o"].Amount != 4000 {
		t.Errorf("adj[o] = %d, want +4000", adj["o"].Amount)
	}
	if adj["r"].Amount != -4000 {
		t.Errorf("adj[r] = %d, want -4000", adj["r"].Amount)
	}

	netted := NetTransactions(txns, links)
	if netted[0].Amount.Amount != -8000 {
		t.Errorf("netted original = %d, want -8000 (net spend)", netted[0].Amount.Amount)
	}
	if netted[1].Amount.Amount != 0 {
		t.Errorf("netted refund = %d, want 0 (no phantom income)", netted[1].Amount.Amount)
	}
	// Input is not mutated.
	if txns[0].Amount.Amount != -12000 || txns[1].Amount.Amount != 4000 {
		t.Errorf("NetTransactions mutated its input")
	}
}

func TestNetAdjustmentsOrderGroupNoOp(t *testing.T) {
	txns := []domain.Transaction{
		txn("a", "Store", -4000, day(2026, 3, 1)),
		txn("b", "Store", -6000, day(2026, 3, 2)),
	}
	links := []domain.TxnLink{{ID: "g", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b"}}}
	if adj := NetAdjustments(txns, links); len(adj) != 0 {
		t.Errorf("order-group produced adjustments %+v, want none", adj)
	}
}

func TestRefundCandidates(t *testing.T) {
	refund := txn("r", "Jacket Co", 12000, day(2026, 4, 5))
	txns := []domain.Transaction{
		refund,
		txn("orig-exact", "Jacket Co", -12000, day(2026, 3, 10)), // exact match, in window
		txn("orig-bigger", "Jacket Co", -15000, day(2026, 3, 1)), // original ≥ refund, ok
		txn("too-small", "Jacket Co", -8000, day(2026, 3, 5)),    // original < refund, excluded
		txn("other-payee", "Shoe Co", -12000, day(2026, 3, 8)),   // wrong payee
		txn("too-old", "Jacket Co", -12000, day(2025, 12, 1)),    // outside 90d window
		txn("future", "Jacket Co", -12000, day(2026, 4, 20)),     // after the refund
	}
	got := RefundCandidates(refund, txns, nil)
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(got), ids(got))
	}
	// Closest amount first: exact match ranks above the bigger original.
	if got[0].ID != "orig-exact" || got[1].ID != "orig-bigger" {
		t.Errorf("candidate order = %v, want [orig-exact orig-bigger]", ids(got))
	}
}

func TestRefundCandidatesExcludesLinkedOriginals(t *testing.T) {
	refund := txn("r", "Jacket Co", 12000, day(2026, 4, 5))
	txns := []domain.Transaction{
		refund,
		txn("orig", "Jacket Co", -12000, day(2026, 3, 10)),
	}
	links := []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"orig", "other"}}}
	if got := RefundCandidates(refund, txns, links); len(got) != 0 {
		t.Errorf("already-linked original surfaced as candidate: %v", ids(got))
	}
}

func ids(ts []domain.Transaction) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}
