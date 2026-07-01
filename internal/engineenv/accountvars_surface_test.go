// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddAccountVarsSurface(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accts := []domain.Account{
		{ID: "a1", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(100000), BalanceAsOf: now},
		{ID: "a2", Name: "Visa", VarName: "card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(-25000), BalanceAsOf: now},
	}
	// A cleared +$200 and an uncleared +$50 into checking.
	txns := []domain.Transaction{
		{ID: "t1", AccountID: "a1", Date: now, Amount: usd(20000), Cleared: true},
		{ID: "t2", AccountID: "a1", Date: now, Amount: usd(5000)},
	}
	vars := Vars(Data{
		Accounts: accts, Transactions: txns, Rates: currency.Rates{Base: "USD"}, Now: now,
	})

	want := map[string]float64{
		"account_checking_balance": 1250, // 1000 opening + 200 cleared + 50 uncleared
		"account_checking_cleared": 1200, // 1000 opening + 200 cleared
		"account_card_balance":     -250, // explicit VarName "card"
	}
	for name, exp := range want {
		if got, ok := vars[name]; !ok {
			t.Errorf("missing surface var %q", name)
		} else if got != exp {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
	// The name-derived slug must NOT be used when VarName is set.
	if _, ok := vars["account_visa_balance"]; ok {
		t.Error("explicit VarName should override the name-derived slug")
	}
}

func TestAccountVarBasesCollision(t *testing.T) {
	accts := []domain.Account{
		{ID: "a1", Name: "Savings"},
		{ID: "a2", Name: "Savings"},
	}
	bases := AccountVarBases(accts)
	if len(bases) != 2 || bases[0].Prefix != "account_savings_" || bases[1].Prefix != "account_savings_2_" {
		t.Errorf("collision handling wrong: %+v", bases)
	}
}
