// SPDX-License-Identifier: MIT

package cashflow

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func txn(minor int64, cur string, date time.Time) domain.Transaction {
	return domain.Transaction{Amount: money.New(minor, cur), Date: date}
}

func TestTrailingMonthly(t *testing.T) {
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 0, 0, 0, 0, time.UTC) }
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		// Trailing whole months are April, May, June (the current month, July, is excluded).
		txn(600000, "USD", d(time.April, 1)),  // income
		txn(-300000, "USD", d(time.April, 5)), // expense
		txn(600000, "USD", d(time.May, 1)),
		txn(-300000, "USD", d(time.May, 5)),
		txn(600000, "USD", d(time.June, 1)),
		txn(-300000, "USD", d(time.June, 5)),
		// A transfer must be ignored entirely (moves money, not income/expense).
		{Amount: money.New(-999999, "USD"), Date: d(time.June, 10), TransferAccountID: "acct-2"},
		// The current month is outside the trailing window and must not count.
		txn(999999, "USD", d(time.July, 2)),
		// Too old (before April) — outside the 3-month window.
		txn(999999, "USD", d(time.March, 2)),
	}
	inc, exp := TrailingMonthly(txns, rates, "USD", now, DefaultTrailingMonths)
	if inc != 600000 {
		t.Fatalf("avg income = %d, want 600000", inc)
	}
	if exp != 300000 {
		t.Fatalf("avg expense = %d, want 300000", exp)
	}
	if got := TrailingMonthlySurplus(txns, rates, "USD", now, DefaultTrailingMonths); got != 300000 {
		t.Fatalf("surplus = %d, want 300000", got)
	}
}

func TestTrailingMonthlySurplusNegative(t *testing.T) {
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 0, 0, 0, 0, time.UTC) }
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		txn(100000, "USD", d(time.June, 1)),
		txn(-450000, "USD", d(time.June, 5)),
	}
	// Only one month has activity but it averages over 3 → still negative.
	if got := TrailingMonthlySurplus(txns, rates, "USD", now, DefaultTrailingMonths); got >= 0 {
		t.Fatalf("surplus = %d, want negative", got)
	}
}

func TestTrailingMonthlyClampsMonths(t *testing.T) {
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	// months < 1 is clamped to 1 (June only).
	txns := []domain.Transaction{txn(500000, "USD", time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC))}
	inc, _ := TrailingMonthly(txns, rates, "USD", now, 0)
	if inc != 500000 {
		t.Fatalf("income over clamped 1-month window = %d, want 500000", inc)
	}
}
