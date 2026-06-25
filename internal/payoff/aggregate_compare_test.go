// SPDX-License-Identifier: MIT

package payoff

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// usdRates builds a Rates table with USD as the base and any extra pairs provided.
func usdRates(extras map[string]float64) currency.Rates {
	r := currency.Rates{Base: "USD", Rates: make(map[string]float64)}
	for k, v := range extras {
		r.Rates[k] = v
	}
	return r
}

// liabilityAccount is a convenience constructor for a minimal liability account.
func liabilityAccount(id, name, cur string, openingMinor int64, apr float64, minPayMinor int64) domain.Account {
	return domain.Account{
		ID:              id,
		Name:            name,
		Class:           domain.ClassLiability,
		Type:            domain.TypeCreditCard,
		Currency:        cur,
		OpeningBalance:  money.New(openingMinor, cur),
		InterestRateAPR: apr,
		MinPayment:      money.New(minPayMinor, cur),
	}
}

// ── AggregateDebts tests ─────────────────────────────────────────────────────

func TestAggregateDebts(t *testing.T) {
	t.Parallel()

	rates := usdRates(map[string]float64{"EUR": 1.08})

	tests := []struct {
		name           string
		accounts       []domain.Account
		txns           []domain.Transaction
		base           string
		rates          currency.Rates
		wantLen        int
		wantMissing    []string
		wantBalance    int64   // first debt's balance in base currency (when wantLen >= 1)
		wantMinPayment int64   // first debt's min payment in base
		wantAPR        float64 // first debt's APR
	}{
		{
			name: "USD debt, no transactions, identity conversion",
			accounts: []domain.Account{
				liabilityAccount("a1", "Visa", "USD", -50000, 19.99, 2500),
			},
			base:           "USD",
			rates:          rates,
			wantLen:        1,
			wantBalance:    50000, // abs of -50000
			wantMinPayment: 2500,
			wantAPR:        19.99,
		},
		{
			name: "EUR debt converted to USD base (1 EUR = 1.08 USD)",
			// Opening balance -100_00 EUR (−€100.00) → owed $108.00 = 10800 cents
			accounts: []domain.Account{
				liabilityAccount("a1", "EUR Card", "EUR", -10000, 15.0, 500),
			},
			base:  "USD",
			rates: rates,
			// ConvertBetween(10000 EUR-cents, "EUR", "USD", rates)
			// = 100 EUR × 1.08 = 108 USD = 10800 USD-cents
			wantLen:     1,
			wantBalance: 10800,
			// min payment: ConvertBetween(500 EUR-cents, "EUR", "USD")
			// = 5 EUR × 1.08 = 5.40 USD = 540 USD-cents
			wantMinPayment: 540,
			wantAPR:        15.0,
		},
		{
			name: "archived account excluded",
			accounts: []domain.Account{
				func() domain.Account {
					a := liabilityAccount("a1", "Old Card", "USD", -10000, 18.0, 500)
					a.Archived = true
					return a
				}(),
			},
			base:    "USD",
			rates:   rates,
			wantLen: 0,
		},
		{
			name: "mortgage excluded by default (IncludedInPayoff == false)",
			accounts: []domain.Account{
				{
					ID:              "m1",
					Name:            "Home Mortgage",
					Class:           domain.ClassLiability,
					Type:            domain.TypeMortgage,
					Currency:        "USD",
					OpeningBalance:  money.New(-30000000, "USD"),
					InterestRateAPR: 3.5,
					MinPayment:      money.New(150000, "USD"),
				},
			},
			base:    "USD",
			rates:   rates,
			wantLen: 0,
		},
		{
			name: "asset account excluded",
			accounts: []domain.Account{
				{
					ID:       "s1",
					Name:     "Savings",
					Class:    domain.ClassAsset,
					Type:     domain.TypeSavings,
					Currency: "USD",
				},
			},
			base:    "USD",
			rates:   rates,
			wantLen: 0,
		},
		{
			name: "unknown currency appended to missingRates, account skipped",
			accounts: []domain.Account{
				liabilityAccount("a1", "BTC Card", "BTC", -100000, 20.0, 5000),
				liabilityAccount("a2", "USD Card", "USD", -20000, 15.0, 1000),
			},
			base:           "USD",
			rates:          rates,
			wantLen:        1,    // only USD card included
			wantMissing:    []string{"BTC"},
			wantBalance:    20000,
			wantMinPayment: 1000,
			wantAPR:        15.0,
		},
		{
			name: "missing rate deduped across multiple accounts in same currency",
			accounts: []domain.Account{
				liabilityAccount("a1", "GBP Card 1", "GBP", -5000, 20.0, 250),
				liabilityAccount("a2", "GBP Card 2", "GBP", -8000, 22.0, 400),
			},
			base:        "USD",
			rates:       rates,  // no GBP rate
			wantLen:     0,
			wantMissing: []string{"GBP"}, // reported once, not twice
		},
		{
			name: "zero-balance account excluded",
			accounts: []domain.Account{
				liabilityAccount("a1", "Paid Off", "USD", 0, 19.99, 0),
			},
			base:    "USD",
			rates:   rates,
			wantLen: 0,
		},
		{
			name: "transaction reduces balance",
			accounts: []domain.Account{
				liabilityAccount("a1", "Visa", "USD", -60000, 18.0, 3000),
			},
			txns: []domain.Transaction{
				{
					ID:        "t1",
					AccountID: "a1",
					Date:      time.Now(),
					Amount:    money.New(10000, "USD"), // payment of $100 reduces owed
				},
			},
			base:           "USD",
			rates:          rates,
			wantLen:        1,
			wantBalance:    50000, // 600 - 100 = 500 USD = 50000 cents
			wantMinPayment: 3000,
			wantAPR:        18.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			debts, missing := AggregateDebts(tc.accounts, tc.txns, tc.base, tc.rates)

			if len(debts) != tc.wantLen {
				t.Errorf("len(debts) = %d, want %d", len(debts), tc.wantLen)
			}
			if !stringSlicesEqual(missing, tc.wantMissing) {
				t.Errorf("missingRates = %v, want %v", missing, tc.wantMissing)
			}
			if tc.wantLen >= 1 && len(debts) >= 1 {
				d := debts[0]
				if d.Balance != tc.wantBalance {
					t.Errorf("debts[0].Balance = %d, want %d", d.Balance, tc.wantBalance)
				}
				if d.MinPayment != tc.wantMinPayment {
					t.Errorf("debts[0].MinPayment = %d, want %d", d.MinPayment, tc.wantMinPayment)
				}
				if d.AprPercent != tc.wantAPR {
					t.Errorf("debts[0].AprPercent = %v, want %v", d.AprPercent, tc.wantAPR)
				}
			}
		})
	}
}

// ── Compare tests ────────────────────────────────────────────────────────────

func TestCompare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		snowball           Plan
		avalanche          Plan
		wantMonthsSaved    int
		wantInterestSaved  int64
		wantFaster         string
	}{
		{
			name:              "tie — both plans identical",
			snowball:          Plan{Months: 24, TotalInterest: 5000},
			avalanche:         Plan{Months: 24, TotalInterest: 5000},
			wantMonthsSaved:   0,
			wantInterestSaved: 0,
			wantFaster:        "tie",
		},
		{
			name:              "avalanche wins on both time and interest",
			snowball:          Plan{Months: 30, TotalInterest: 8000},
			avalanche:         Plan{Months: 24, TotalInterest: 6000},
			wantMonthsSaved:   6,    // snowball.Months - avalanche.Months
			wantInterestSaved: 2000, // snowball.TotalInterest - avalanche.TotalInterest
			wantFaster:        "avalanche",
		},
		{
			name:              "snowball wins on time (unusual scenario)",
			snowball:          Plan{Months: 20, TotalInterest: 9000},
			avalanche:         Plan{Months: 25, TotalInterest: 7000},
			wantMonthsSaved:   -5,   // negative: snowball is faster
			wantInterestSaved: 2000, // avalanche would still cost less interest
			wantFaster:        "snowball",
		},
		{
			name:              "same months, avalanche saves interest",
			snowball:          Plan{Months: 24, TotalInterest: 7000},
			avalanche:         Plan{Months: 24, TotalInterest: 5500},
			wantMonthsSaved:   0,
			wantInterestSaved: 1500,
			wantFaster:        "avalanche",
		},
		{
			name:              "same months, snowball saves interest (unusual)",
			snowball:          Plan{Months: 24, TotalInterest: 5000},
			avalanche:         Plan{Months: 24, TotalInterest: 5500},
			wantMonthsSaved:   0,
			wantInterestSaved: -500, // negative: avalanche actually costs more
			wantFaster:        "snowball",
		},
		{
			name:              "avalanche faster by one month",
			snowball:          Plan{Months: 25, TotalInterest: 6000},
			avalanche:         Plan{Months: 24, TotalInterest: 5000},
			wantMonthsSaved:   1,
			wantInterestSaved: 1000,
			wantFaster:        "avalanche",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Compare(tc.snowball, tc.avalanche)
			if got.MonthsSaved != tc.wantMonthsSaved {
				t.Errorf("MonthsSaved = %d, want %d", got.MonthsSaved, tc.wantMonthsSaved)
			}
			if got.InterestSavedMinor != tc.wantInterestSaved {
				t.Errorf("InterestSavedMinor = %d, want %d", got.InterestSavedMinor, tc.wantInterestSaved)
			}
			if got.Faster != tc.wantFaster {
				t.Errorf("Faster = %q, want %q", got.Faster, tc.wantFaster)
			}
		})
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// stringSlicesEqual reports whether two string slices have the same elements in
// the same order, treating nil and empty as equal.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
