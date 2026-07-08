// SPDX-License-Identifier: MIT

package valuation

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func snap(id string, minor int64, y int, m time.Month, d int) domain.BalanceSnapshot {
	return domain.BalanceSnapshot{ID: id, BalanceMinor: minor, AsOf: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

func TestMonthToDateChange(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }

	tests := []struct {
		name    string
		snaps   []domain.BalanceSnapshot
		current money.Money
		wantOK  bool
		want    int64
	}{
		{
			name:   "no snapshots",
			snaps:  nil,
			wantOK: false,
		},
		{
			// Baseline is the last snapshot at/before July 1 (the June 30 one, 30000),
			// carried forward; current 32000 → +2000.
			name: "carry-forward from before month start",
			snaps: []domain.BalanceSnapshot{
				snap("a", 28000_00, 2026, time.May, 1),
				snap("b", 30000_00, 2026, time.June, 30),
				snap("c", 31000_00, 2026, time.July, 10),
			},
			current: usd(32000_00),
			wantOK:  true,
			want:    2000_00,
		},
		{
			// History began this month (no snapshot <= July 1): baseline is the
			// earliest snapshot (26000 on July 6); current 24000 → -2000.
			name: "fallback to earliest when all within month",
			snaps: []domain.BalanceSnapshot{
				snap("a", 26000_00, 2026, time.July, 6),
				snap("b", 24000_00, 2026, time.July, 6),
			},
			current: usd(24000_00),
			wantOK:  true,
			want:    -2000_00,
		},
		{
			// A snapshot exactly on the first of the month counts as the baseline.
			name: "snapshot on the first is the baseline",
			snaps: []domain.BalanceSnapshot{
				snap("a", 50000_00, 2026, time.July, 1),
				snap("b", 55000_00, 2026, time.July, 12),
			},
			current: usd(55000_00),
			wantOK:  true,
			want:    5000_00,
		},
		{
			name: "no change",
			snaps: []domain.BalanceSnapshot{
				snap("a", 10000_00, 2026, time.June, 1),
			},
			current: usd(10000_00),
			wantOK:  true,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := MonthToDateChange(tt.snaps, tt.current, now)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Amount != tt.want {
				t.Errorf("change = %d, want %d", got.Amount, tt.want)
			}
			if got.Currency != "USD" {
				t.Errorf("currency = %q, want USD", got.Currency)
			}
		})
	}
}

func TestMonthToDateChangeUnsorted(t *testing.T) {
	// Snapshots given newest-first must yield the same result as oldest-first.
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	snaps := []domain.BalanceSnapshot{
		snap("c", 31000_00, 2026, time.July, 10),
		snap("a", 28000_00, 2026, time.May, 1),
		snap("b", 30000_00, 2026, time.June, 30),
	}
	got, ok := MonthToDateChange(snaps, money.New(32000_00, "USD"), now)
	if !ok || got.Amount != 2000_00 {
		t.Fatalf("got (%d, %v), want (200000, true)", got.Amount, ok)
	}
}
