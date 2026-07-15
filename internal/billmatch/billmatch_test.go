// SPDX-License-Identifier: MIT

package billmatch

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestIsCandidate(t *testing.T) {
	occ := Occurrence{
		RecurringID: "r1", DueDate: d(2026, 7, 15),
		Payee: "Netflix", CategoryID: "cat-ent", AmountMinor: 1599, Currency: "USD",
	}
	tests := []struct {
		name string
		txn  Txn
		want bool
	}{
		{"exact payee+amount+date", Txn{ID: "t1", Date: d(2026, 7, 15), Payee: "Netflix", AmountMinor: -1599, Currency: "USD"}, true},
		{"within 5% over", Txn{ID: "t2", Date: d(2026, 7, 16), Payee: "Netflix", AmountMinor: -1650, Currency: "USD"}, true},
		{"amount too far", Txn{ID: "t3", Date: d(2026, 7, 15), Payee: "Netflix", AmountMinor: -2500, Currency: "USD"}, false},
		{"date too far", Txn{ID: "t4", Date: d(2026, 7, 25), Payee: "Netflix", AmountMinor: -1599, Currency: "USD"}, false},
		{"date within window early", Txn{ID: "t5", Date: d(2026, 7, 11), Payee: "Netflix", AmountMinor: -1599, Currency: "USD"}, true},
		{"identity by category only", Txn{ID: "t6", Date: d(2026, 7, 15), Payee: "Some Other", CategoryID: "cat-ent", AmountMinor: -1599, Currency: "USD"}, true},
		{"no identity match", Txn{ID: "t7", Date: d(2026, 7, 15), Payee: "Spotify", CategoryID: "cat-x", AmountMinor: -1599, Currency: "USD"}, false},
		{"currency mismatch", Txn{ID: "t8", Date: d(2026, 7, 15), Payee: "Netflix", AmountMinor: -1599, Currency: "EUR"}, false},
	}
	for _, tc := range tests {
		if got := isCandidate(occ, tc.txn); got != tc.want {
			t.Errorf("%s: isCandidate = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSmallBillTolerance(t *testing.T) {
	// 5% of $2 = 10c, but the floor is $1, so a $2.80 charge for a $2 bill matches.
	occ := Occurrence{RecurringID: "r", DueDate: d(2026, 7, 1), Payee: "Tip", AmountMinor: 200, Currency: "USD"}
	txn := Txn{ID: "t", Date: d(2026, 7, 1), Payee: "Tip", AmountMinor: -280, Currency: "USD"}
	if !isCandidate(occ, txn) {
		t.Fatal("small bill within $1 floor should match")
	}
}

func TestAutoMatchesUnambiguous(t *testing.T) {
	occs := []Occurrence{
		{RecurringID: "r1", DueDate: d(2026, 7, 15), Payee: "Netflix", AmountMinor: 1599, Currency: "USD"},
		{RecurringID: "r2", DueDate: d(2026, 7, 20), Payee: "Rent", AmountMinor: 200000, Currency: "USD"},
	}
	txns := []Txn{
		{ID: "t1", Date: d(2026, 7, 15), Payee: "Netflix", AmountMinor: -1650, Currency: "USD"},
		{ID: "t2", Date: d(2026, 7, 19), Payee: "Rent", AmountMinor: -200000, Currency: "USD"},
		{ID: "t3", Date: d(2026, 7, 1), Payee: "Grocery", AmountMinor: -5000, Currency: "USD"},
	}
	got := AutoMatches(occs, txns, nil)
	if len(got) != 2 {
		t.Fatalf("want 2 matches, got %d: %+v", len(got), got)
	}
	if got[0].RecurringID != "r1" || got[0].TxnID != "t1" || got[0].VarianceMinor != 51 {
		t.Errorf("match0 = %+v, want r1/t1 variance 51", got[0])
	}
	if got[1].RecurringID != "r2" || got[1].VarianceMinor != 0 {
		t.Errorf("match1 = %+v, want r2 variance 0", got[1])
	}
}

func TestAutoMatchesAmbiguousLeftAlone(t *testing.T) {
	// Two identical Netflix charges both match the one occurrence — ambiguous.
	occs := []Occurrence{
		{RecurringID: "r1", DueDate: d(2026, 7, 15), Payee: "Netflix", AmountMinor: 1599, Currency: "USD"},
	}
	txns := []Txn{
		{ID: "t1", Date: d(2026, 7, 14), Payee: "Netflix", AmountMinor: -1599, Currency: "USD"},
		{ID: "t2", Date: d(2026, 7, 16), Payee: "Netflix", AmountMinor: -1599, Currency: "USD"},
	}
	if got := AutoMatches(occs, txns, nil); len(got) != 0 {
		t.Fatalf("ambiguous occurrence should not auto-match, got %+v", got)
	}
}

func TestAutoMatchesContendedTxn(t *testing.T) {
	// One txn is the sole candidate of two occurrences — contended, not matched.
	occs := []Occurrence{
		{RecurringID: "r1", DueDate: d(2026, 7, 15), Payee: "Gym", AmountMinor: 3000, Currency: "USD"},
		{RecurringID: "r2", DueDate: d(2026, 7, 16), Payee: "Gym", AmountMinor: 3000, Currency: "USD"},
	}
	txns := []Txn{
		{ID: "t1", Date: d(2026, 7, 15), Payee: "Gym", AmountMinor: -3000, Currency: "USD"},
	}
	if got := AutoMatches(occs, txns, nil); len(got) != 0 {
		t.Fatalf("contended txn should not auto-match, got %+v", got)
	}
}

func TestAutoMatchesSkipsAlreadyMatched(t *testing.T) {
	occs := []Occurrence{
		{RecurringID: "r1", DueDate: d(2026, 7, 15), Payee: "Netflix", AmountMinor: 1599, Currency: "USD"},
	}
	txns := []Txn{
		{ID: "t1", Date: d(2026, 7, 15), Payee: "Netflix", AmountMinor: -1599, Currency: "USD"},
	}
	already := map[string]string{Key("r1", d(2026, 7, 15)): "t1"}
	if got := AutoMatches(occs, txns, already); len(got) != 0 {
		t.Fatalf("already-matched occurrence should be skipped, got %+v", got)
	}
}

func TestKeyMatchesDomainFormat(t *testing.T) {
	if got := Key("r1", time.Date(2026, 3, 1, 13, 0, 0, 0, time.UTC)); got != "r1|2026-03-01" {
		t.Fatalf("Key = %q", got)
	}
}
