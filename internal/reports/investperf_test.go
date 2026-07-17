// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestInvestmentPerformance(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "brk", Name: "Brokerage", Type: domain.TypeInvestment, OpeningBalance: money.New(500000, "USD")},   // $5,000 opening
		{ID: "chk", Name: "Checking", Type: domain.TypeChecking, OpeningBalance: money.New(100000, "USD")},      // skipped (not investment)
		{ID: "old", Name: "Old", Type: domain.TypeInvestment, Archived: true, OpeningBalance: money.New(1, "USD")}, // skipped (archived)
	}
	txns := []domain.Transaction{
		// $2,000 transferred IN (a contribution).
		{ID: "t1", AccountID: "brk", Amount: money.New(200000, "USD"), TransferAccountID: "chk", Date: day},
		// $300 dividend (non-transfer income).
		{ID: "t2", AccountID: "brk", Amount: money.New(30000, "USD"), Date: day},
		// $700 value update / market gain (non-transfer).
		{ID: "t3", AccountID: "brk", Amount: money.New(70000, "USD"), Date: day},
	}
	perf, err := InvestmentPerformance(accounts, txns, rates)
	if err != nil {
		t.Fatalf("InvestmentPerformance: %v", err)
	}
	if len(perf) != 1 {
		t.Fatalf("got %d rows, want 1 (only the active brokerage): %+v", len(perf), perf)
	}
	p := perf[0]
	// Invested = $5,000 opening + $2,000 transfer in = $7,000.
	if p.Invested != 700000 {
		t.Errorf("Invested = %d, want 700000 ($7,000)", p.Invested)
	}
	// Current = $5,000 + $2,000 + $300 + $700 = $8,000.
	if p.Current != 800000 {
		t.Errorf("Current = %d, want 800000 ($8,000)", p.Current)
	}
	// Gain = $8,000 - $7,000 = $1,000 (the dividend + value update).
	if p.Gain != 100000 {
		t.Errorf("Gain = %d, want 100000 ($1,000)", p.Gain)
	}
	// Return = 1000 / 7000 = 14.28% ≈ 1428 bips.
	if p.ReturnBips != 1428 {
		t.Errorf("ReturnBips = %d, want 1428 (~14.28%%)", p.ReturnBips)
	}
}

func TestInvestmentPerformanceNoBasis(t *testing.T) {
	// An account with zero cost basis (funded only by gains) reports 0% return, no panic.
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{{ID: "x", Name: "X", Type: domain.TypeCrypto}}
	txns := []domain.Transaction{{ID: "g", AccountID: "x", Amount: money.New(5000, "USD"), Date: time.Now()}}
	perf, err := InvestmentPerformance(accounts, txns, rates)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(perf) != 1 || perf[0].ReturnBips != 0 {
		t.Errorf("want one row with 0 bips (no basis), got %+v", perf)
	}
}
