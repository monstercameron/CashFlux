// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestHealthInputsExcludeUnconvertibleAmounts: when an account's currency has
// no FX rate, its figures are EXCLUDED from the health inputs — never added as
// raw minor units masquerading as base currency (¥50,000 is not $50,000).
func TestHealthInputsExcludeUnconvertibleAmounts(t *testing.T) {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(rates map[string]float64) Data {
		return Data{
			Accounts: []domain.Account{
				{ID: "chk", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
					OpeningBalance: money.New(500000, "USD"), BalanceAsOf: asOf},
				// USD loan: min payment $200/mo — always convertible.
				{ID: "loan", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD",
					OpeningBalance: money.New(-1000000, "USD"), BalanceAsOf: asOf,
					MinPayment: money.New(20000, "USD")},
				// JPY loan: min payment ¥50,000 (5,000,000 minor units).
				{ID: "jpy", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "JPY",
					OpeningBalance: money.New(-100000000, "JPY"), BalanceAsOf: asOf,
					MinPayment: money.New(5000000, "JPY")},
				// EUR credit card: balance and limit need a EUR rate.
				{ID: "eur-card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "EUR",
					OpeningBalance: money.New(-50000, "EUR"), BalanceAsOf: asOf,
					CreditLimit: money.New(100000, "EUR")},
				// USD credit card: $300 owed on a $1,000 limit → 30% on its own.
				{ID: "usd-card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
					OpeningBalance: money.New(-30000, "USD"), BalanceAsOf: asOf,
					CreditLimit: money.New(100000, "USD")},
			},
			Transactions: []domain.Transaction{
				// Steady income over the trailing full months so HasIncome is true.
				{ID: "i1", AccountID: "chk", Date: now.AddDate(0, -1, 0), Amount: money.New(300000, "USD")},
				{ID: "i2", AccountID: "chk", Date: now.AddDate(0, -2, 0), Amount: money.New(300000, "USD")},
				{ID: "i3", AccountID: "chk", Date: now.AddDate(0, -3, 5), Amount: money.New(300000, "USD")},
			},
			Rates: currency.Rates{Base: "USD", Rates: rates},
			Now:   now,
		}
	}

	// No JPY/EUR rates: the unconvertible figures must drop out entirely.
	in := HealthInputs(mk(map[string]float64{}))
	if !in.HasIncome {
		t.Fatal("fixture should have income")
	}
	// Only the $200 USD payment counts against the $3,000 monthly income
	// ($9,000 over the trailing 3 full months): 200/3000 = 6%. The raw-fallback
	// bug would have added ¥50,000 as $50,000 and blown this past 1000%.
	if in.ObligationRatioPct != 6 {
		t.Errorf("ObligationRatioPct = %d, want 6 (JPY payment excluded, not added raw)", in.ObligationRatioPct)
	}
	// Only the USD card counts: 300/1000 = 30%.
	if in.AggUtilizationPct != 30 {
		t.Errorf("AggUtilizationPct = %d, want 30 (EUR card excluded, not added raw)", in.AggUtilizationPct)
	}

	// With rates present, the foreign figures participate (sanity: ratios move).
	withRates := HealthInputs(mk(map[string]float64{"JPY": 150, "EUR": 0.9}))
	if withRates.ObligationRatioPct <= in.ObligationRatioPct {
		t.Errorf("with a JPY rate the obligation ratio should rise: %d vs %d",
			withRates.ObligationRatioPct, in.ObligationRatioPct)
	}
	if withRates.AggUtilizationPct == in.AggUtilizationPct {
		t.Error("with a EUR rate the utilization should include the EUR card")
	}
}
