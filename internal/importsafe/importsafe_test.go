// SPDX-License-Identifier: MIT

package importsafe

import (
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/money"
)

func txn(id, acct, desc string, minor int64, date string) domain.Transaction {
	d, _ := time.Parse("2006-01-02", date)
	return domain.Transaction{ID: id, AccountID: acct, Desc: desc, Date: d, Amount: money.New(minor, "USD")}
}

func TestDecimalCommaMappingPreviewMatchesImporterDuplicateCount(t *testing.T) {
	mapped, errs := importmap.ApplyWithErrors(importmap.Profile{
		DateCol: 0, DescCol: 1, AmountCol: 2, DebitCol: -1, CreditCol: -1,
		BalanceCol: -1, Decimals: 2, NumberFormat: importmap.NumberFormatCommaDecimal,
	}, [][]string{
		{"2026-07-01", "Cafe", "(12,50)"},
		{"2026-07-01", "Cafe", "(12,50)"},
	})
	if len(errs) != 0 || len(mapped) != 2 {
		t.Fatalf("mapping rows=%d errors=%+v", len(mapped), errs)
	}
	incoming := make([]domain.Transaction, 0, len(mapped))
	for _, row := range mapped {
		incoming = append(incoming, domain.Transaction{
			AccountID: "acc", Date: row.Date, Desc: row.Description, Amount: money.New(row.Amount, "EUR"),
		})
	}
	preview := Duplicates(incoming, nil, "acc")
	importer := dedupe.CountIncomingDuplicates(incoming, nil, "acc")
	if len(preview) != importer || importer != 0 {
		t.Fatalf("preview duplicates=%d, importer duplicates=%d; same-file repeats should both import", len(preview), importer)
	}
	if incoming[0].Amount.Amount != -1250 {
		t.Errorf("decimal-comma amount = %d, want -1250 EUR minor units", incoming[0].Amount.Amount)
	}
}

func TestImpact(t *testing.T) {
	rows := []domain.Transaction{
		txn("1", "", "coffee", -500, "2026-07-01"),
		txn("2", "a1", "salary", 100000, "2026-07-02"),
		txn("3", "OTHER", "elsewhere", -99999, "2026-07-03"), // different account — excluded
	}
	net, after, err := Impact(25000, rows, "a1", "USD")
	if err != nil {
		t.Fatal(err)
	}
	if net != 99500 || after != 124500 {
		t.Fatalf("Impact = net %d after %d", net, after)
	}
	if net, after, err = Impact(0, nil, "a1", "USD"); err != nil || net != 0 || after != 0 {
		t.Fatalf("empty Impact = %d/%d", net, after)
	}
}

func TestImpactRejectsMixedCurrencyAndOverflow(t *testing.T) {
	euro := txn("e", "a1", "EUR", 100, "2026-07-01")
	euro.Amount.Currency = "EUR"
	_, _, err := Impact(0, []domain.Transaction{euro}, "a1", "USD")
	if err == nil {
		t.Fatal("mixed-currency import was summed as USD")
	}
	_, _, err = Impact(1, []domain.Transaction{txn("e", "a1", "USD", int64(^uint64(0)>>1), "2026-07-01")}, "a1", "USD")
	if err == nil {
		t.Fatal("balance overflow was accepted")
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

func TestJumpWarningHandlesMinimumIntegerWithoutOverflow(t *testing.T) {
	if !JumpWarning(0, -1<<63) {
		t.Fatal("minimum int64 magnitude should exceed the warning floor")
	}
	if JumpWarning(1<<62, -1<<63) {
		t.Fatal("exactly twice the balance should not trigger the three-times warning")
	}
}

func TestDuplicatesMirrorsDedupeCountAndExplains(t *testing.T) {
	existing := []domain.Transaction{txn("e1", "a1", "Netflix", -1599, "2026-07-01")}
	incoming := []domain.Transaction{
		txn("", "", "NETFLIX ", -1599, "2026-07-01"),  // vs ledger (case/space-insensitive)
		txn("", "", "gym", -3000, "2026-07-02"),       // fresh
		txn("", "", "gym", -3000, "2026-07-02"),       // repeats within the batch
		txn("", "a2", "Netflix", -1599, "2026-07-01"), // other account — NOT a dup
	}
	dups := Duplicates(incoming, existing, "a1")
	if len(dups) != dedupe.CountIncomingDuplicates(incoming, existing, "a1") {
		t.Fatalf("detail count %d differs from importer count %d: %+v", len(dups), dedupe.CountIncomingDuplicates(incoming, existing, "a1"), dups)
	}
	if len(dups) != 1 {
		t.Fatalf("got %d dups, want 1: %+v", len(dups), dups)
	}
	if dups[0].Desc != "NETFLIX" || dups[0].AmountMinor != -1599 || dups[0].Date != "2026-07-01" || dups[0].InBatch {
		t.Errorf("ledger dup detail wrong: %+v", dups[0])
	}
	if dups[0].InBatch {
		t.Errorf("a match against the existing ledger should not be flagged InBatch: %+v", dups[0])
	}
	if got := Duplicates(incoming[1:3], nil, "a1"); len(got) != 0 {
		t.Errorf("same-file repeats with no matching existing row must both import: %+v", got)
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

func BenchmarkImport3000Rows(b *testing.B) {
	incoming := make([]domain.Transaction, 3000)
	existing := make([]domain.Transaction, 3000)
	for i := range incoming {
		incoming[i] = txn("in"+strconv.Itoa(i), "card", "payment", 100, "2026-07-03")
		existing[i] = txn("ex"+strconv.Itoa(i), "checking", "transfer", -100, "2026-07-01")
	}
	b.Run("before-quadratic", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkNaiveTransferPairs(incoming, existing)
		}
	})
	b.Run("after-indexed", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = TransferPairs(incoming, existing, "card", 4)
		}
	})
}

func benchmarkNaiveTransferPairs(incoming, existing []domain.Transaction) {
	claimed := make(map[string]bool, len(incoming))
	for _, in := range incoming {
		for _, ex := range existing {
			if !claimed[ex.ID] && ex.Amount.Amount == -in.Amount.Amount && ex.Amount.Currency == in.Amount.Currency {
				claimed[ex.ID] = true
				break
			}
		}
	}
}
