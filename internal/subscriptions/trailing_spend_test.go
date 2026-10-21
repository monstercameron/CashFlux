// SPDX-License-Identifier: MIT

package subscriptions

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTrailingSpendChecked(t *testing.T) {
	now := time.Date(2025, time.April, 19, 12, 0, 0, 0, time.UTC)
	readErr := errors.New("transaction snapshot read failed")

	for _, tc := range []struct {
		name         string
		transactions []domain.Transaction
		sourceErr    error
		rates        currency.Rates
		want         TrailingSpend
		wantErr      error
	}{
		{
			name: "three prior full months and nonempty month denominator",
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2024, time.December, 31, -40_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.January, 31, -1_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.February, 10, 8_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.March, 31, -2_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.March, 12, -9_000, "USD", "transfer-account"),
				trailingSpendTestTransaction(2025, time.March, 14, -6_000, "USD", "excluded-report"),
				trailingSpendTestTransaction(2025, time.April, 1, -30_000, "USD", ""),
			},
			rates: currency.Rates{Base: "USD"},
			want:  TrailingSpend{TotalExpense: 3_000, EligibleMonths: 2},
		},
		{
			name: "all three completed months contribute",
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2025, time.January, 1, -500, "USD", ""),
				trailingSpendTestTransaction(2025, time.February, 1, -1_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.March, 1, -1_500, "USD", ""),
			},
			rates: currency.Rates{Base: "USD"},
			want:  TrailingSpend{TotalExpense: 3_000, EligibleMonths: 3},
		},
		{
			name: "missing FX rejects the whole denominator",
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2025, time.January, 1, -1_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.March, 1, -500, "EUR", ""),
			},
			rates:   currency.Rates{Base: "USD"},
			wantErr: currency.ErrUnknownRate,
		},
		{
			name: "source read error rejects the supplied rows",
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2025, time.January, 1, -1_000, "USD", ""),
			},
			sourceErr: readErr,
			rates:     currency.Rates{Base: "USD"},
			wantErr:   readErr,
		},
		{
			name: "pooled money overflow rejects partial total",
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2025, time.January, 1, -math.MaxInt64, "USD", ""),
				trailingSpendTestTransaction(2025, time.February, 1, -1, "USD", ""),
			},
			rates:   currency.Rates{Base: "USD"},
			wantErr: money.ErrOverflow,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TrailingSpendChecked(tc.transactions, tc.sourceErr, now, tc.rates)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("TrailingSpendChecked error=%v, want errors.Is(_, %v)", err, tc.wantErr)
				}
				if got != (TrailingSpend{}) {
					t.Fatalf("failed preparation returned partial spend %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("TrailingSpendChecked: %v", err)
			}
			if got != tc.want {
				t.Fatalf("TrailingSpendChecked = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestTrailingSpendCheckedAnchorsAtCalendarMonthStart(t *testing.T) {
	easternIndia := time.FixedZone("UTC+05:30", 5*60*60+30*60)
	for _, tc := range []struct {
		name         string
		now          time.Time
		transactions []domain.Transaction
		want         TrailingSpend
	}{
		{
			name: "May 31 includes February through April once",
			now:  time.Date(2025, time.May, 31, 12, 0, 0, 0, time.UTC),
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2025, time.January, 1, -9_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.February, 1, -300, "USD", ""),
				trailingSpendTestTransaction(2025, time.March, 1, -200, "USD", ""),
				trailingSpendTestTransaction(2025, time.April, 1, -100, "USD", ""),
				trailingSpendTestTransaction(2025, time.May, 1, -10_000, "USD", ""),
			},
			want: TrailingSpend{TotalExpense: 600, EligibleMonths: 3},
		},
		{
			name: "leap-year March 31 includes December through February once",
			now:  time.Date(2024, time.March, 31, 12, 0, 0, 0, time.UTC),
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2023, time.November, 1, -7_000, "USD", ""),
				trailingSpendTestTransaction(2023, time.December, 1, -400, "USD", ""),
				trailingSpendTestTransaction(2024, time.January, 1, -500, "USD", ""),
				trailingSpendTestTransaction(2024, time.February, 1, -600, "USD", ""),
				trailingSpendTestTransaction(2024, time.March, 1, -9_000, "USD", ""),
			},
			want: TrailingSpend{TotalExpense: 1_500, EligibleMonths: 3},
		},
		{
			name: "month follows local calendar across UTC boundary",
			now:  time.Date(2025, time.May, 1, 0, 30, 0, 0, easternIndia),
			transactions: []domain.Transaction{
				trailingSpendTestTransaction(2025, time.January, 1, -1_000, "USD", ""),
				trailingSpendTestTransaction(2025, time.February, 1, -200, "USD", ""),
				trailingSpendTestTransaction(2025, time.March, 1, -300, "USD", ""),
				trailingSpendTestTransaction(2025, time.April, 30, -400, "USD", ""),
				trailingSpendTestTransaction(2025, time.May, 1, -9_000, "USD", ""),
			},
			want: TrailingSpend{TotalExpense: 900, EligibleMonths: 3},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TrailingSpendChecked(tc.transactions, nil, tc.now, currency.Rates{Base: "USD"})
			if err != nil {
				t.Fatalf("TrailingSpendChecked: %v", err)
			}
			if got != tc.want {
				t.Fatalf("TrailingSpendChecked = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func trailingSpendTestTransaction(year int, month time.Month, day int, amount int64, code, marker string) domain.Transaction {
	txn := domain.Transaction{
		Date:   time.Date(year, month, day, 12, 0, 0, 0, time.UTC),
		Amount: money.New(amount, code),
	}
	switch marker {
	case "transfer-account":
		txn.TransferAccountID = "other"
	case "excluded-report":
		txn.ExcludeFromReports = true
	}
	return txn
}
