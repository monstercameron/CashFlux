// SPDX-License-Identifier: MIT

package billmatch

import (
	"math"
	"testing"
)

func TestBillMatchHardening(t *testing.T) {
	occ := Occurrence{
		RecurringID: "r1", DueDate: d(2026, 7, 15),
		Payee: "Netflix", CategoryID: "cat-ent", AmountMinor: 1599, Currency: "USD",
	}

	// Positive refund must never match an expense bill occurrence.
	if isCandidate(occ, Txn{ID: "ref", Date: d(2026, 7, 15), Payee: "Netflix", AmountMinor: +1599, Currency: "USD"}) {
		t.Fatal("positive refund matched expense bill")
	}

	// 1-2 char payee noise must not substring-match unrelated merchants.
	if isCandidate(occ, Txn{ID: "noise", Date: d(2026, 7, 15), Payee: "X", AmountMinor: -1599, Currency: "USD"}) {
		t.Fatal("1-char payee substring matched Netflix")
	}
	// Exact short payee still matches itself.
	shortOcc := Occurrence{RecurringID: "r2", DueDate: d(2026, 7, 15), Payee: "X", AmountMinor: 1599, Currency: "USD"}
	if !isCandidate(shortOcc, Txn{ID: "exact", Date: d(2026, 7, 15), Payee: "x", AmountMinor: -1599, Currency: "USD"}) {
		t.Fatal("exact short payee should still match")
	}

	// Case-insensitive currency comparison.
	if !isCandidate(occ, Txn{ID: "lower", Date: d(2026, 7, 15), Payee: "Netflix", AmountMinor: -1599, Currency: " usd "}) {
		t.Fatal("normalized currency should match")
	}

	// MinInt64 and far-apart years must not overflow or falsely match.
	bigOcc := Occurrence{RecurringID: "big", DueDate: d(2026, 7, 15), Payee: "Big", AmountMinor: math.MaxInt64, Currency: "USD"}
	if isCandidate(bigOcc, Txn{ID: "far", Date: d(1700, 1, 1), Payee: "Big", AmountMinor: math.MinInt64, Currency: "USD"}) {
		t.Fatal("far-apart year matched due to duration overflow")
	}

	// Contended txn: occ1 has [t1], occ2 has [t1, t2] — t1 is contended and must NOT auto-match occ1.
	occs := []Occurrence{
		{RecurringID: "r1", DueDate: d(2026, 7, 15), Payee: "Gym", AmountMinor: 3000, Currency: "USD"},
		{RecurringID: "r2", DueDate: d(2026, 7, 18), Payee: "Gym", AmountMinor: 3000, Currency: "USD"},
	}
	txns := []Txn{
		{ID: "t1", Date: d(2026, 7, 15), Payee: "Gym", AmountMinor: -3000, Currency: "USD"},
		{ID: "t2", Date: d(2026, 7, 21), Payee: "Gym", AmountMinor: -3000, Currency: "USD"},
	}
	if got := AutoMatches(occs, txns, nil); len(got) != 0 {
		t.Fatalf("contended txn t1 auto-matched despite second occurrence claiming it: %+v", got)
	}
}
