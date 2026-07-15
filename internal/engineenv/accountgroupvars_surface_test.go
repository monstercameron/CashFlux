// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddGroupVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: usd(300000)},
		{ID: "card", Name: "Visa", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: usd(-100000)},
	}
	groups := []AccountGroupDef{{Name: "Shared", AccountIDs: []string{"chk", "card"}}}
	vars := Vars(Data{Accounts: accts, Groups: groups, Rates: currency.Rates{Base: "USD"}, Now: now})

	// Net subtotal = 3000 asset − 1000 liability magnitude = 2000 (major units).
	if got := vars["group_shared_total"]; got != 2000 {
		t.Errorf("group_shared_total = %v, want 2000", got)
	}
}

func TestGroupVarBasesCollision(t *testing.T) {
	bases := GroupVarBases([]AccountGroupDef{{Name: "Cash"}, {Name: "Cash"}})
	if len(bases) != 2 || bases[0].Prefix != "group_cash_" || bases[1].Prefix != "group_cash_2_" {
		t.Errorf("collision handling wrong: %+v", bases)
	}
}

func TestAddAccountFlowVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
	}
	inPeriod := func(d int) time.Time { return time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC) }
	txns := []domain.Transaction{
		{ID: "i", AccountID: "chk", Amount: usd(200000), Date: inPeriod(3)},
		{ID: "e", AccountID: "chk", Amount: usd(-50000), Date: inPeriod(10)},
		{ID: "t", AccountID: "chk", TransferAccountID: "sav", Amount: usd(-99900), Date: inPeriod(11)},
	}
	vars := Vars(Data{Accounts: accts, Transactions: txns, Rates: currency.Rates{Base: "USD"}, Now: now})
	if got := vars["account_checking_in"]; got != 2000 {
		t.Errorf("account_checking_in = %v, want 2000", got)
	}
	if got := vars["account_checking_out"]; got != 500 {
		t.Errorf("account_checking_out = %v, want 500 (transfer excluded)", got)
	}
}
