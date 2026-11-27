// SPDX-License-Identifier: MIT

package txnlinks

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTxnLinksHardening(t *testing.T) {
	// Reconcile must not report balanced across mismatched currencies.
	if _, bal := Reconcile(money.New(-10000, "USD"), money.New(10000, "EUR")); bal {
		t.Fatal("Reconcile reported balanced for USD vs EUR")
	}

	// GroupSum skips mismatched currencies.
	sum := GroupSum([]domain.Transaction{
		{ID: "a", Amount: money.New(-4000, "USD")},
		{ID: "b", Amount: money.New(-5000, "EUR")},
		{ID: "c", Amount: money.New(-1000, "usd")},
	})
	if sum.Amount != -5000 || sum.Currency != "USD" {
		t.Fatalf("GroupSum = %+v, want -5000 USD", sum)
	}

	// Cumulative cap across two partial refunds must never flip original into income.
	orig := txn("o", "Shop", -10000, day(2026, 3, 10))
	r1 := txn("r1", "Shop", 8000, day(2026, 3, 12))
	r2 := txn("r2", "Shop", 8000, day(2026, 3, 13))
	links := []domain.TxnLink{
		{ID: "l1", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"o", "r1"}},
		{ID: "l2", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"o", "r2"}},
	}
	netted := NetTransactions([]domain.Transaction{orig, r1, r2}, links)
	if netted[0].Amount.Amount != 0 {
		t.Fatalf("over-refunded original = %d, want 0 (never phantom income)", netted[0].Amount.Amount)
	}
	if netted[1].Amount.Amount != 0 || netted[2].Amount.Amount != 6000 {
		t.Fatalf("refund balances = %d, %d; want 0, 6000", netted[1].Amount.Amount, netted[2].Amount.Amount)
	}

	// RefundNet ignores non-expense original and non-income refund.
	incOrig := txn("io", "Shop", 5000, day(2026, 3, 10))
	expRef := txn("er", "Shop", -5000, day(2026, 3, 11))
	if got := RefundNet(domain.TxnLink{Kind: domain.TxnLinkRefundPair}, incOrig, r1); got.Amount != 0 {
		t.Fatalf("RefundNet accepted income original: %+v", got)
	}
	if got := RefundNet(domain.TxnLink{Kind: domain.TxnLinkRefundPair}, orig, expRef); got.Amount != 0 {
		t.Fatalf("RefundNet accepted expense refund: %+v", got)
	}

	// NetTransactions scales Splits proportionally so Σ(Splits) == Amount holds.
	splitOrig := domain.Transaction{
		ID: "so", Payee: "Mall", Date: day(2026, 3, 10), Amount: usd(-10000),
		Splits: []domain.CategorySplit{
			{CategoryID: "c1", Amount: usd(-6000)},
			{CategoryID: "c2", Amount: usd(-4000)},
		},
	}
	splitRef := txn("sr", "Mall", 2500, day(2026, 3, 15))
	splitNet := NetTransactions([]domain.Transaction{splitOrig, splitRef}, []domain.TxnLink{
		{ID: "sl", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"so", "sr"}},
	})
	if !splitNet[0].SplitsReconcile() {
		t.Fatalf("netted splits do not reconcile: amt=%d splits=%+v", splitNet[0].Amount.Amount, splitNet[0].Splits)
	}
	if splitNet[0].Splits[0].Amount.Amount != -4500 || splitNet[0].Splits[1].Amount.Amount != -3000 {
		t.Fatalf("scaled splits = %+v, want -4500 and -3000", splitNet[0].Splits)
	}
	// Original input splits must not be mutated.
	if splitOrig.Splits[0].Amount.Amount != -6000 {
		t.Fatal("NetTransactions mutated caller's Splits slice")
	}

	// Blank payee/desc must not match in RefundCandidates.
	blankRef := txn("br", "", 1000, day(2026, 4, 1))
	blankOrig := txn("bo", "", -1000, day(2026, 3, 20))
	if got := RefundCandidates(blankRef, []domain.Transaction{blankRef, blankOrig}, nil); len(got) != 0 {
		t.Fatalf("blank-key refund matched blank-key purchase: %+v", got)
	}
}
