// SPDX-License-Identifier: MIT

package importsafe

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func txn(id, acct, desc string, minor int64, date string) domain.Transaction {
	d, _ := time.Parse("2006-01-02", date)
	return domain.Transaction{ID: id, AccountID: acct, Desc: desc, Date: d, Amount: money.New(minor, "USD")}
}

func TestImpact(t *testing.T) {
	rows := []domain.Transaction{
		txn("1", "", "coffee", -500, "2026-07-01"),
		txn("2", "a1", "salary", 100000, "2026-07-02"),
		txn("3", "OTHER", "elsewhere", -99999, "2026-07-03"), // different account — excluded
	}
	net, after := Impact(25000, rows, "a1")
	if net != 99500 || after != 124500 {
		t.Fatalf("Impact = net %d after %d", net, after)
	}
	if net, after = Impact(0, nil, "a1"); net != 0 || after != 0 {
		t.Fatalf("empty Impact = %d/%d", net, after)
	}
}

func TestJumpWarning(t *testing.T) {
	cases := []struct {
		name         string
		current, net int64
		want         bool
	}{
		{"small import never warns", 0, 50_000, false},
		{"under floor even on empty account", 0, 999_999, false},
		{"big import into empty account", 0, 1_000_000, true},
		{"big but proportionate", 4_000_000, 1_200_000, false},
		{"big and disproportionate", 90_000, 1_200_000, true},
		{"negative net symmetrical", 90_000, -1_200_000, true},
		{"negative balance uses magnitude", -4_000_000, 1_200_000, false},
		{"exactly 3x is allowed", 400_000, 1_200_000, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JumpWarning(tc.current, tc.net); got != tc.want {
				t.Errorf("JumpWarning(%d, %d) = %v, want %v", tc.current, tc.net, got, tc.want)
			}
		})
	}
}

func TestDuplicatesMirrorsDedupeCountAndExplains(t *testing.T) {
	existing := []domain.Transaction{txn("e1", "a1", "Netflix", -1599, "2026-07-01")}
	incoming := []domain.Transaction{
		txn("", "", "NETFLIX ", -1599, "2026-07-01"), // vs ledger (case/space-insensitive)
		txn("", "", "gym", -3000, "2026-07-02"),      // fresh
		txn("", "", "gym", -3000, "2026-07-02"),      // repeats within the batch
		txn("", "a2", "Netflix", -1599, "2026-07-01"), // other account — NOT a dup
	}
	dups := Duplicates(incoming, existing, "a1")
	if len(dups) != 2 {
		t.Fatalf("got %d dups, want 2: %+v", len(dups), dups)
	}
	if dups[0].Desc != "NETFLIX" || dups[0].AmountMinor != -1599 || dups[0].Date != "2026-07-01" || dups[0].InBatch {
		t.Errorf("ledger dup detail wrong: %+v", dups[0])
	}
	if dups[1].Desc != "gym" || !dups[1].InBatch {
		t.Errorf("batch dup should be flagged InBatch: %+v", dups[1])
	}
}

func TestTransferPairs(t *testing.T) {
	existing := []domain.Transaction{
		txn("e1", "chk", "Transfer to card", -25000, "2026-07-01"),
		txn("e2", "chk", "groceries", -8000, "2026-07-01"),
		txn("e3", "sav", "far away", -25000, "2026-05-01"), // outside window
	}
	incoming := []domain.Transaction{
		txn("", "", "PAYMENT THANK YOU", 25000, "2026-07-03"), // pairs with e1 (2 days)
		txn("", "", "PAYMENT THANK YOU", 25000, "2026-07-03"), // e1 already claimed → no pair
		txn("", "", "interest", 12, "2026-07-03"),             // no mirror
	}
	pairs := TransferPairs(incoming, existing, "card", 4)
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want 1: %+v", len(pairs), pairs)
	}
	p := pairs[0]
	if p.OtherAccount != "chk" || p.OtherDesc != "Transfer to card" || p.AmountMinor != 25000 || p.IncomingDate != "2026-07-03" {
		t.Errorf("pair detail wrong: %+v", p)
	}
	// Same-account mirrors are refunds, not transfers.
	if got := TransferPairs([]domain.Transaction{txn("", "", "refund", 8000, "2026-07-02")},
		existing, "chk", 4); len(got) != 0 {
		t.Errorf("same-account mirror must not pair: %+v", got)
	}
	// Rows already marked as transfers are skipped.
	tr := txn("", "", "xfer", 25000, "2026-07-02")
	tr.TransferAccountID = "chk"
	if got := TransferPairs([]domain.Transaction{tr}, existing, "card", 4); len(got) != 0 {
		t.Errorf("transfer rows must be skipped: %+v", got)
	}
}
