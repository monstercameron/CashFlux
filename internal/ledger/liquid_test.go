// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func acct(id string, ty domain.AccountType, opening int64, archived bool) domain.Account {
	return domain.Account{ID: id, Type: ty, Currency: "USD", OpeningBalance: money.New(opening, "USD"), Archived: archived}
}

func TestLiquidBalance(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{
		acct("chk", domain.TypeChecking, 100000, false),
		acct("sav", domain.TypeSavings, 50000, false),
		acct("inv", domain.TypeInvestment, 999999, false), // not liquid
		acct("cc", domain.TypeCreditCard, -20000, false),  // liability, not liquid
		acct("oldcash", domain.TypeCash, 80000, true),     // archived → excluded
		acct("wallet", domain.TypeCash, 2500, false),      // liquid
	}
	got, err := LiquidBalance(accounts, nil, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// chk 100000 + sav 50000 + wallet 2500 = 152500.
	if got.Amount != 152500 || got.Currency != "USD" {
		t.Errorf("LiquidBalance = %v, want 152500 USD", got)
	}
}

func TestLiquidBalanceWithTransactions(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{acct("chk", domain.TypeChecking, 10000, false)}
	txns := []domain.Transaction{
		{AccountID: "chk", Amount: money.New(-3000, "USD")},
		{AccountID: "chk", Amount: money.New(500, "USD")},
		{AccountID: "other", Amount: money.New(-9999, "USD")}, // different account, ignored
	}
	got, _ := LiquidBalance(accounts, txns, rates)
	// 10000 - 3000 + 500 = 7500.
	if got.Amount != 7500 {
		t.Errorf("LiquidBalance = %d, want 7500", got.Amount)
	}
}

func TestLiquidBalanceEmpty(t *testing.T) {
	got, err := LiquidBalance(nil, nil, currency.Rates{Base: "USD"})
	if err != nil || got.Amount != 0 || got.Currency != "USD" {
		t.Errorf("empty = %v err=%v, want 0 USD", got, err)
	}
}
