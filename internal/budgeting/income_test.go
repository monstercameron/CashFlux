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

// incomeTxnCat is incomeTxn with a CategoryID, for the by-category income basis.
func incomeTxnCat(dateDay int, amountMinor int64, curr, catID string) domain.Transaction {
	t := incomeTxn(dateDay, amountMinor, curr)
	t.CategoryID = catID
	return t
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

func TestZeroBasedIncome(t *testing.T) {
	rates := makeRates(nil)
	start, end := jan(1), jan(31)
	txns := []domain.Transaction{
		incomeTxnCat(3, 300000, "USD", "salary"),   // $3000 paycheck
		incomeTxnCat(10, 5000, "USD", "freelance"), // $50 side hustle
		incomeTxnCat(17, 300000, "USD", "salary"),  // $3000 paycheck
		expenseTxn(5, 10000, "USD"),                // ignored (expense)
		transferTxn(6, 100000, "USD"),              // ignored (transfer)
	}
	cases := []struct {
		name       string
		mode       string
		paycheckMn int64
		configured int64
		cats       []string
		want       int64
	}{
		{"all sums every deposit", IncomeModeAll, 0, 0, nil, 605000},
		{"paychecks drops sub-threshold side income", IncomeModePaychecks, 10000, 0, nil, 600000},
		{"paychecks with no threshold == all", IncomeModePaychecks, 0, 0, nil, 605000},
		{"fixed uses configured, ignores txns", IncomeModeFixed, 0, 500000, nil, 500000},
		{"fixed unset returns 0", IncomeModeFixed, 0, 0, nil, 0},
		{"unknown mode falls back to all", "bogus", 0, 0, nil, 605000},
		{"categories sums only chosen sources", IncomeModeCategories, 0, 0, []string{"salary"}, 600000},
		{"categories can add a side source back", IncomeModeCategories, 0, 0, []string{"salary", "freelance"}, 605000},
		{"categories with no source chosen is zero", IncomeModeCategories, 0, 0, nil, 0},
		{"categories ignores unmatched category", IncomeModeCategories, 0, 0, []string{"bonus"}, 0},
	}
	for _, tc := range cases {
		if got := ZeroBasedIncome(tc.mode, tc.paycheckMn, tc.configured, tc.cats, txns, start, end, "USD", rates); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestAveragedIncome(t *testing.T) {
	rates := makeRates(nil)
	monthStart := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	mk := func(y int, m time.Month, minor int64) domain.Transaction {
		return domain.Transaction{ID: "t", Date: time.Date(y, m, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(minor, "USD"), CategoryID: "salary"}
	}
	txns := []domain.Transaction{
		mk(2023, time.October, 300000),  // Oct — in the 3-month window
		mk(2023, time.November, 360000), // Nov
		mk(2023, time.December, 240000), // Dec (last month)
		mk(2024, time.January, 999999),  // Jan — at/after monthStart, excluded (half-open)
	}
	cases := []struct {
		name   string
		mode   string
		cfg    int64
		cats   []string
		months int
		want   int64
	}{
		{"3-mo average of all income", IncomeModeAll, 0, nil, 3, 300000},     // (3000+3600+2400)/3
		{"months<1 means last month only", IncomeModeAll, 0, nil, 0, 240000}, // Dec only
		{"categories averaged over 3 months", IncomeModeCategories, 0, []string{"salary"}, 3, 300000},
		{"fixed is never averaged", IncomeModeFixed, 500000, nil, 3, 500000},
	}
	for _, tc := range cases {
		if got := AveragedIncome(tc.mode, 0, tc.cfg, tc.cats, txns, monthStart, tc.months, "USD", rates); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
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
