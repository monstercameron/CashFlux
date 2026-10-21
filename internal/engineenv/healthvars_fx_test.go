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

	// No JPY/EUR rates: neither ratio may claim the convertible subset is complete.
	in := HealthInputs(mk(map[string]float64{}))
	if !in.HasIncome {
		t.Fatal("fixture should have income")
	}
	if in.Unavailable["debt"] == nil || in.Unavailable["utilization"] == nil {
		t.Fatalf("partial foreign totals remained available: %+v", in)
	}
	if in.HasCredit {
		t.Fatal("partial card set must not establish household utilization")
	}
	// Independent income/savings still use complete same-currency transactions.
	if !in.HasIncome || in.SavingsRatePct != 100 || in.Unavailable["savings"] != nil {
		t.Fatalf("foreign account failure removed independent cash flow: %+v", in)
	}

	// With rates present, both complete aggregates become available again.
	withRates := HealthInputs(mk(map[string]float64{"JPY": 150, "EUR": 0.9}))
	if withRates.Unavailable["debt"] != nil || withRates.ObligationRatioPct <= 0 {
		t.Errorf("with a JPY rate the obligation ratio should rise: %d vs %d",
			withRates.ObligationRatioPct, in.ObligationRatioPct)
	}
	if withRates.Unavailable["utilization"] != nil || !withRates.HasCredit || withRates.AggUtilizationPct <= 0 {
		t.Error("with a EUR rate the utilization should include the EUR card")
	}
}
