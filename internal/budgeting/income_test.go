// SPDX-License-Identifier: MIT

package budgeting

import (
	"maps"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// makeRates builds a Rates table with USD as base and optional additional
// currencies mapped to their USD value per unit.
func makeRates(extras map[string]float64) currency.Rates {
	r := currency.Rates{
		Base:  "USD",
		Rates: make(map[string]float64),
	}
	maps.Copy(r.Rates, extras)
	return r
}

// jan builds a UTC time in January 2024 for test fixtures.
func jan(day int) time.Time {
	return time.Date(2024, time.January, day, 0, 0, 0, 0, time.UTC)
}

// incomeTxn returns an income Transaction (positive, non-transfer) on the given
// date in the given currency code.
func incomeTxn(dateDay int, amountMinor int64, curr string) domain.Transaction {
	return domain.Transaction{
		ID:     "t-inc",
		Date:   jan(dateDay),
		Amount: money.New(amountMinor, curr),
	}
}

// expenseTxn returns an expense Transaction (negative, non-transfer).
func expenseTxn(dateDay int, amountMinor int64, curr string) domain.Transaction {
	return domain.Transaction{
		ID:     "t-exp",
		Date:   jan(dateDay),
		Amount: money.New(-amountMinor, curr),
	}
}

// transferTxn returns a transfer Transaction (TransferAccountID set).
func transferTxn(dateDay int, amountMinor int64, curr string) domain.Transaction {
	return domain.Transaction{
		ID:                "t-xfer",
		Date:              jan(dateDay),
		Amount:            money.New(amountMinor, curr),
		TransferAccountID: "acc-other",
	}
}

func TestIncomeForBudgets(t *testing.T) {
	rates := makeRates(map[string]float64{
		"EUR": 1.10, // 1 EUR = 1.10 USD
	})

	start := jan(1)
	end := jan(31) // half-open: [Jan 1, Jan 31)

	tests := []struct {
		name            string
		configuredMinor int64
		txns            []domain.Transaction
		wantMinor       int64
	}{
		{
			name:            "configured>0 returns configured, ignores txns",
			configuredMinor: 500_00, // $500.00
			txns: []domain.Transaction{
				incomeTxn(15, 1000_00, "USD"), // $1000 income in window — must be ignored
			},
			wantMinor: 500_00,
		},
		{
			name:            "configured==0 sums actual income in window",
			configuredMinor: 0,
			txns: []domain.Transaction{
				incomeTxn(5, 300_00, "USD"),  // $300 in window
				incomeTxn(10, 200_00, "USD"), // $200 in window
			},
			wantMinor: 500_00, // $500 total
		},
		{
			name:            "out-of-window txns excluded",
			configuredMinor: 0,
			txns: []domain.Transaction{
				incomeTxn(5, 300_00, "USD"),  // Jan  5 — in window
				incomeTxn(31, 999_00, "USD"), // Jan 31 — at end boundary, excluded (half-open)
			},
			wantMinor: 300_00,
		},
		{
			name:            "expense txns not counted as income",
			configuredMinor: 0,
			txns: []domain.Transaction{
				incomeTxn(5, 400_00, "USD"),  // income
				expenseTxn(6, 150_00, "USD"), // expense — must not reduce the income sum
			},
			wantMinor: 400_00,
		},
		{
			name:            "transfer txns excluded from income",
			configuredMinor: 0,
			txns: []domain.Transaction{
				incomeTxn(5, 400_00, "USD"),   // genuine income
				transferTxn(6, 250_00, "USD"), // transfer — IsIncome() returns false, must be excluded
			},
			wantMinor: 400_00,
		},
		{
			name:            "FX conversion applied for non-base income txn",
			configuredMinor: 0,
			txns: []domain.Transaction{
				// 100 EUR × 1.10 = 110 USD (minor: 110_00 cents)
				incomeTxn(10, 100_00, "EUR"),
			},
			wantMinor: 110_00,
		},
		{
			name:            "configured<0 treated as 0 — falls back to actual",
			configuredMinor: -1,
			txns: []domain.Transaction{
				incomeTxn(5, 250_00, "USD"),
			},
			wantMinor: 250_00,
		},
		{
			name:            "no txns and configured==0 returns zero",
			configuredMinor: 0,
			txns:            nil,
			wantMinor:       0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IncomeForBudgets(tc.configuredMinor, tc.txns, start, end, "USD", rates)
			if got != tc.wantMinor {
				t.Errorf("IncomeForBudgets() = %d, want %d", got, tc.wantMinor)
			}
		})
	}
}
