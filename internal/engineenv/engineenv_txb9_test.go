// SPDX-License-Identifier: MIT

package engineenv

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestVarsBalanceAggregationUnavailableWhenAccountFoldFails(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	data := Data{
		Accounts: []domain.Account{
			{ID: "valid", Name: "Valid", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(12500, "USD")},
			{ID: "invalid", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: money.New(20000, "USD")},
		},
		Transactions: []domain.Transaction{{ID: "bad-currency", AccountID: "invalid", Date: now, Amount: money.New(100, "EUR")}},
		Rates:        currency.Rates{Base: "USD", Rates: map[string]float64{}},
		Now:          now,
	}

	vars := Vars(data)
	for _, name := range []string{"assets", "liabilities", "liquid_cash", "net_worth"} {
		if value, ok := vars[name]; ok {
			t.Errorf("%s = %v, present=%v; want unavailable because an account balance failed", name, value, ok)
		}
	}
	if got := vars["account_valid_balance"]; got != 125 {
		t.Errorf("valid account balance = %v, want 125", got)
	}
}

func TestVarsBalanceAggregationUnavailableOnOverflow(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	data := Data{
		Accounts:     []domain.Account{{ID: "overflow", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")}},
		Transactions: []domain.Transaction{{ID: "overflow-tx", AccountID: "overflow", Date: now, Amount: money.New(1, "USD")}},
		Rates:        currency.Rates{Base: "USD", Rates: map[string]float64{}},
		Now:          now,
	}

	vars := Vars(data)
	for _, name := range []string{"assets", "liabilities", "liquid_cash", "net_worth"} {
		if value, ok := vars[name]; ok {
			t.Errorf("%s = %v, present=%v; want unavailable because account balance overflowed", name, value, ok)
		}
	}
}
