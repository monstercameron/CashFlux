// SPDX-License-Identifier: MIT

// Tests for the pure helpers in roundup.go (roundUpDue and roundUpTotal).
// No build tag — runs on native Go without syscall/js.
package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRoundUpDue(t *testing.T) {
	tests := []struct {
		name       string
		lastPeriod string
		nowKey     string
		want       bool
	}{
		{name: "never run (empty lastPeriod)", lastPeriod: "", nowKey: "2026-06", want: true},
		{name: "ran in a prior month", lastPeriod: "2026-05", nowKey: "2026-06", want: true},
		{name: "already ran this month", lastPeriod: "2026-06", nowKey: "2026-06", want: false},
		{name: "same strings", lastPeriod: "2025-01", nowKey: "2025-01", want: false},
		{name: "different year", lastPeriod: "2025-12", nowKey: "2026-01", want: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := roundUpDue(tc.lastPeriod, tc.nowKey)
			if got != tc.want {
				t.Errorf("roundUpDue(%q, %q) = %v; want %v", tc.lastPeriod, tc.nowKey, got, tc.want)
			}
		})
	}
}

func TestRoundUpTotal(t *testing.T) {
	// June 2026 range.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// Helper to build an expense transaction on accID with the given amount (negative).
	expense := func(accID string, amountMinor int64, date time.Time) domain.Transaction {
		return domain.Transaction{
			AccountID: accID,
			Amount:    money.New(amountMinor, "USD"),
			Date:      date,
		}
	}
	// Helper to build an income transaction.
	income := func(accID string, amountMinor int64, date time.Time) domain.Transaction {
		return domain.Transaction{
			AccountID: accID,
			Amount:    money.New(amountMinor, "USD"),
			Date:      date,
		}
	}
	// Helper to build a transfer transaction (has TransferAccountID set).
	transfer := func(accID string, amountMinor int64, date time.Time) domain.Transaction {
		return domain.Transaction{
			AccountID:         accID,
			Amount:            money.New(amountMinor, "USD"),
			Date:              date,
			TransferAccountID: "other-acc",
		}
	}

	juneDate := func(day int) time.Time {
		return time.Date(2026, 6, day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name          string
		txns          []domain.Transaction
		fromAccountID string
		gran          int64
		start, end    time.Time
		want          int64
	}{
		{
			name: "sums deltas for expenses on correct account",
			txns: []domain.Transaction{
				expense("acc1", -347, juneDate(5)),  // 347 → 400, delta = 53
				expense("acc1", -500, juneDate(10)), // exact dollar, delta = 0
				expense("acc1", -201, juneDate(15)), // 201 → 300, delta = 99
			},
			fromAccountID: "acc1",
			gran:          100,
			start:         start,
			end:           end,
			want:          152, // 53 + 0 + 99
		},
		{
			name: "skips expenses on wrong account",
			txns: []domain.Transaction{
				expense("acc1", -347, juneDate(5)),
				expense("acc2", -201, juneDate(5)), // different account
			},
			fromAccountID: "acc1",
			gran:          100,
			start:         start,
			end:           end,
			want:          53,
		},
		{
			name: "skips income transactions",
			txns: []domain.Transaction{
				expense("acc1", -347, juneDate(5)),
				income("acc1", 1000, juneDate(10)), // positive = income, skip
			},
			fromAccountID: "acc1",
			gran:          100,
			start:         start,
			end:           end,
			want:          53,
		},
		{
			name: "skips transfer transactions",
			txns: []domain.Transaction{
				expense("acc1", -347, juneDate(5)),
				transfer("acc1", -200, juneDate(10)), // transfer, skip
			},
			fromAccountID: "acc1",
			gran:          100,
			start:         start,
			end:           end,
			want:          53,
		},
		{
			name: "skips transactions outside the date range",
			txns: []domain.Transaction{
				expense("acc1", -347, juneDate(5)),
				expense("acc1", -201, time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)), // May, out of range
				expense("acc1", -150, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),  // July (end is exclusive), skip
			},
			fromAccountID: "acc1",
			gran:          100,
			start:         start,
			end:           end,
			want:          53,
		},
		{
			name: "zero granularity defaults to 100",
			txns: []domain.Transaction{
				expense("acc1", -347, juneDate(5)), // delta = 53
			},
			fromAccountID: "acc1",
			gran:          0, // should default to 100
			start:         start,
			end:           end,
			want:          53,
		},
		{
			name: "nearest $5 granularity (500 minor)",
			txns: []domain.Transaction{
				expense("acc1", -347, juneDate(5)),  // 347 → 500, delta = 153
				expense("acc1", -500, juneDate(10)), // exact $5, delta = 0
				expense("acc1", -501, juneDate(15)), // 501 → 1000, delta = 499
			},
			fromAccountID: "acc1",
			gran:          500,
			start:         start,
			end:           end,
			want:          652, // 153 + 0 + 499
		},
		{
			name:          "no transactions yields zero",
			txns:          nil,
			fromAccountID: "acc1",
			gran:          100,
			start:         start,
			end:           end,
			want:          0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := roundUpTotal(tc.txns, tc.fromAccountID, tc.gran, tc.start, tc.end)
			if got != tc.want {
				t.Errorf("roundUpTotal(...) = %d; want %d", got, tc.want)
			}
		})
	}
}
