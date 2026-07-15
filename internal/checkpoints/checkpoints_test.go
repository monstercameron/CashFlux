// SPDX-License-Identifier: MIT

package checkpoints

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func txn(acc string, d time.Time, minor int64) domain.Transaction {
	return domain.Transaction{AccountID: acc, Date: d, Amount: money.Money{Amount: minor, Currency: "USD"}}
}

func acct() domain.Account {
	return domain.Account{ID: "a1", Currency: "USD", OpeningBalance: money.Money{Amount: 100_00, Currency: "USD"}}
}

func TestLedgerFoldWithoutCheckpoints(t *testing.T) {
	a := acct()
	txns := []domain.Transaction{
		txn("a1", day(2026, 1, 5), 50_00),
		txn("a1", day(2026, 1, 10), -20_00),
		txn("a2", day(2026, 1, 6), 999_00), // other account ignored
	}
	got := BalanceMinorAt(a, txns, nil, day(2026, 1, 15))
	if want := int64(130_00); got != want {
		t.Errorf("fold = %d, want %d", got, want)
	}
}

func TestNearestAnchorWins(t *testing.T) {
	a := acct()
	// Sparse ledger: only one txn recorded, but a checkpoint corrects reality.
	txns := []domain.Transaction{
		txn("a1", day(2026, 3, 20), -30_00),
	}
	cps := ForAccount([]domain.BalanceSnapshot{
		{AccountID: "a1", AsOf: day(2026, 3, 1), BalanceMinor: 5000_00},
		{AccountID: "a1", AsOf: day(2026, 1, 1), BalanceMinor: 1000_00},
	}, "a1")
	// After the March anchor: 5000 + (-30) = 4970.
	if got := BalanceMinorAt(a, txns, cps, day(2026, 3, 25)); got != 4970_00 {
		t.Errorf("post-anchor = %d, want %d", got, 4970_00)
	}
	// Between Jan and Mar anchors, before the txn: Jan anchor, no txns since.
	if got := BalanceMinorAt(a, txns, cps, day(2026, 2, 15)); got != 1000_00 {
		t.Errorf("mid-anchor = %d, want %d", got, 1000_00)
	}
	// Before any anchor: ledger fold (opening 100 only).
	if got := BalanceMinorAt(a, txns, cps, day(2025, 12, 1)); got != 100_00 {
		t.Errorf("pre-anchor = %d, want %d", got, 100_00)
	}
}

func TestAnchorOnSameDayCountsTxnAfter(t *testing.T) {
	a := acct()
	txns := []domain.Transaction{
		txn("a1", day(2026, 3, 1), 200_00), // same day as anchor — folded in, never counted
		txn("a1", day(2026, 3, 2), 50_00),  // day after anchor — counted from 3/2 on
	}
	cps := ForAccount([]domain.BalanceSnapshot{
		{AccountID: "a1", AsOf: day(2026, 3, 1), BalanceMinor: 1000_00},
	}, "a1")
	// Same-day txn is NOT counted (anchor is end-of-day confirmed); balance = anchor.
	if got := BalanceMinorAt(a, txns, cps, day(2026, 3, 1)); got != 1000_00 {
		t.Errorf("same-day = %d, want %d", got, 1000_00)
	}
	// Next day counts only the 3/2 txn, not the same-day 3/1 one.
	if got := BalanceMinorAt(a, txns, cps, day(2026, 3, 2)); got != 1050_00 {
		t.Errorf("next-day = %d, want %d", got, 1050_00)
	}
}

func TestSeries(t *testing.T) {
	a := acct()
	cps := ForAccount([]domain.BalanceSnapshot{
		{AccountID: "a1", AsOf: day(2026, 1, 1), BalanceMinor: 500_00},
	}, "a1")
	txns := []domain.Transaction{txn("a1", day(2026, 1, 3), -100_00)}
	s := Series(a, txns, cps, day(2026, 1, 1), day(2026, 1, 4))
	if len(s) != 4 {
		t.Fatalf("series len = %d, want 4", len(s))
	}
	wants := []int64{500_00, 500_00, 400_00, 400_00}
	for i, w := range wants {
		if s[i].BalanceMinor != w {
			t.Errorf("day %d balance = %d, want %d", i, s[i].BalanceMinor, w)
		}
	}
	if Series(a, txns, cps, day(2026, 1, 5), day(2026, 1, 1)) != nil {
		t.Error("inverted range should be nil")
	}
}

func TestNetWorthAnchored(t *testing.T) {
	asset := domain.Account{ID: "a1", Currency: "USD", Class: domain.ClassAsset}
	liab := domain.Account{ID: "l1", Currency: "USD", Class: domain.ClassLiability}
	cps := []domain.BalanceSnapshot{
		{AccountID: "a1", AsOf: day(2026, 1, 1), BalanceMinor: 8000_00},
		{AccountID: "l1", AsOf: day(2026, 1, 1), BalanceMinor: 3000_00},
	}
	got := NetWorthMinorAt([]domain.Account{asset, liab}, nil, cps, day(2026, 1, 10))
	if want := int64(5000_00); got != want {
		t.Errorf("net worth = %d, want %d", got, want)
	}
}
