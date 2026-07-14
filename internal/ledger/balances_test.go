// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// bulkFixture returns accounts + transactions exercising the bulk paths:
// multi-account, multi-currency, cleared/uncleared, an unconvertible currency,
// and one account with a corrupt (mismatched-currency) opening balance.
func bulkFixture() ([]domain.Account, []domain.Transaction, currency.Rates) {
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
			OpeningBalance: money.New(100000, "USD"), BalanceAsOf: asOf},
		{ID: "sav", Name: "Savings", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
			OpeningBalance: money.New(500000, "USD"), BalanceAsOf: asOf},
		{ID: "eur", Name: "Euro fund", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "EUR",
			OpeningBalance: money.New(200000, "EUR"), BalanceAsOf: asOf},
		{ID: "card", Name: "Visa", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
			OpeningBalance: money.New(-30000, "USD"), BalanceAsOf: asOf, CreditLimit: money.New(100000, "USD")},
		{ID: "bad", Name: "Corrupt", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
			OpeningBalance: money.New(1000, "GBP"), BalanceAsOf: asOf}, // opening currency mismatch
		{ID: "arch", Name: "Old", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", Archived: true,
			OpeningBalance: money.New(70000, "USD"), BalanceAsOf: asOf},
	}
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "t1", AccountID: "chk", Date: now, Amount: money.New(25000, "USD"), Cleared: true},
		{ID: "t2", AccountID: "chk", Date: now, Amount: money.New(-5000, "USD")},
		{ID: "t3", AccountID: "sav", Date: now, Amount: money.New(10000, "USD"), Cleared: true},
		{ID: "t4", AccountID: "eur", Date: now, Amount: money.New(-20000, "EUR"), Cleared: true},
		{ID: "t5", AccountID: "card", Date: now, Amount: money.New(-15000, "USD")},
		{ID: "t6", AccountID: "ghost", Date: now, Amount: money.New(999, "USD")}, // unknown account
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}} // EUR has NO rate
	return accounts, txns, rates
}

// TestBalancesMatchesPerAccount: the single-pass map agrees with Balance /
// ClearedBalance per account, and errored accounts are absent + reported.
func TestBalancesMatchesPerAccount(t *testing.T) {
	accounts, txns, _ := bulkFixture()

	bals, err := Balances(accounts, txns)
	if err == nil {
		t.Error("Balances should report the corrupt account's error")
	}
	cleared, _ := ClearedBalances(accounts, txns)

	for _, a := range accounts {
		want, perErr := Balance(a, txns)
		got, ok := bals[a.ID]
		if perErr != nil {
			if ok {
				t.Errorf("account %s: bulk has %v but per-account errors (%v)", a.ID, got, perErr)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("account %s: bulk balance %v (present=%v), want %v", a.ID, got, ok, want)
		}
		wantCl, _ := ClearedBalance(a, txns)
		if gotCl := cleared[a.ID]; gotCl != wantCl {
			t.Errorf("account %s: bulk cleared %v, want %v", a.ID, gotCl, wantCl)
		}
	}
}

// TestNetWorthFromBalancesMatchesExplained: identical result to
// NetWorthExplained on the same data, including missing-rate exclusions and
// liability magnitude handling — minus the corrupt account, which both reject.
func TestNetWorthFromBalancesMatchesExplained(t *testing.T) {
	accounts, txns, rates := bulkFixture()
	// Drop the corrupt account: both variants error on it identically.
	clean := accounts[:4]

	want, err := NetWorthExplained(clean, txns, rates)
	if err != nil {
		t.Fatalf("NetWorthExplained: %v", err)
	}
	bals, _ := Balances(clean, txns)
	got, err := NetWorthFromBalances(clean, bals, rates)
	if err != nil {
		t.Fatalf("NetWorthFromBalances: %v", err)
	}
	if got.Net != want.Net || got.Assets != want.Assets || got.Liabilities != want.Liabilities {
		t.Errorf("totals diverge: got %+v, want %+v", got, want)
	}
	if len(got.MissingCurrencies) != 1 || got.MissingCurrencies[0] != "EUR" {
		t.Errorf("MissingCurrencies = %v, want [EUR]", got.MissingCurrencies)
	}

	// The corrupt account errors through both paths.
	if _, err := NetWorthExplained(accounts, txns, rates); err == nil {
		t.Error("NetWorthExplained should error on the corrupt account")
	}
	allBals, _ := Balances(accounts, txns)
	if _, err := NetWorthFromBalances(accounts, allBals, rates); err == nil {
		t.Error("NetWorthFromBalances should error on the corrupt (absent) account")
	}
}

// TestLiquidFromBalancesMatchesLiquidBalance: identical to LiquidBalance —
// cash-type non-archived accounts only, converted to base.
func TestLiquidFromBalancesMatchesLiquidBalance(t *testing.T) {
	accounts, txns, rates := bulkFixture()
	clean := accounts[:4] // corrupt account is cash-type: both variants error on it

	want, err := LiquidBalance(clean, txns, rates)
	if err != nil {
		t.Fatalf("LiquidBalance: %v", err)
	}
	bals, _ := Balances(clean, txns)
	got, err := LiquidFromBalances(clean, bals, rates)
	if err != nil {
		t.Fatalf("LiquidFromBalances: %v", err)
	}
	if got != want {
		t.Errorf("liquid = %v, want %v", got, want)
	}

	if _, err := LiquidBalance(accounts, txns, rates); err == nil {
		t.Error("LiquidBalance should error on the corrupt cash account")
	}
	allBals, _ := Balances(accounts, txns)
	if _, err := LiquidFromBalances(accounts, allBals, rates); err == nil {
		t.Error("LiquidFromBalances should error on the corrupt (absent) cash account")
	}
}
