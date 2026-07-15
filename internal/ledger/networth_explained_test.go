// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func asset(id, name, cur string, openMinor int64) domain.Account {
	return domain.Account{
		ID: id, Name: name, Currency: cur, Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(openMinor, cur),
	}
}

func TestNetWorthExplainedExcludesMissingRate(t *testing.T) {
	accounts := []domain.Account{
		asset("a1", "Checking", "USD", 100000), // $1,000
		asset("a2", "Savings", "EUR", 50000),   // €500 -> $550 at 1.10
		asset("a3", "Brokerage", "GBP", 20000), // £200, NO rate -> excluded
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}

	res, err := NetWorthExplained(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetWorthExplained: %v", err)
	}
	// The GBP account is excluded, NOT counted as base/zero: total = 1000 + 550.
	if res.Net.Amount != 155000 {
		t.Errorf("net = %d, want 155000 (USD + converted EUR, GBP excluded)", res.Net.Amount)
	}
	if res.Assets.Amount != 155000 {
		t.Errorf("assets = %d, want 155000", res.Assets.Amount)
	}
	if len(res.MissingCurrencies) != 1 || res.MissingCurrencies[0] != "GBP" {
		t.Errorf("MissingCurrencies = %v, want [GBP]", res.MissingCurrencies)
	}
	if len(res.ExcludedAccounts) != 1 || res.ExcludedAccounts[0] != "Brokerage" {
		t.Errorf("ExcludedAccounts = %v, want [Brokerage]", res.ExcludedAccounts)
	}
}

func TestNetWorthExplainedAllRatesPresent(t *testing.T) {
	accounts := []domain.Account{
		asset("a1", "Checking", "USD", 100000),
		asset("a2", "Savings", "EUR", 50000),
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	res, err := NetWorthExplained(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetWorthExplained: %v", err)
	}
	if len(res.MissingCurrencies) != 0 || len(res.ExcludedAccounts) != 0 {
		t.Errorf("no rate should be missing: %+v", res)
	}
	if res.Net.Amount != 155000 {
		t.Errorf("net = %d, want 155000", res.Net.Amount)
	}
}

func TestNetWorthExplainedLiabilityExcluded(t *testing.T) {
	loan := domain.Account{
		ID: "l1", Name: "GBP Loan", Currency: "GBP", Type: domain.TypeLoan, Class: domain.ClassLiability,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(-10000, "GBP"),
	}
	accounts := []domain.Account{asset("a1", "Checking", "USD", 100000), loan}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	res, err := NetWorthExplained(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetWorthExplained: %v", err)
	}
	// Only the USD asset counts; the GBP liability is excluded (not silently zeroed).
	if res.Net.Amount != 100000 || res.Liabilities.Amount != 0 {
		t.Errorf("net=%d liab=%d, want net 100000, liab 0 with GBP excluded", res.Net.Amount, res.Liabilities.Amount)
	}
	if len(res.ExcludedAccounts) != 1 {
		t.Errorf("ExcludedAccounts = %v, want [GBP Loan]", res.ExcludedAccounts)
	}
}

func TestNetWorthExplainedExcludesByChoice(t *testing.T) {
	a2 := asset("a2", "Kid's custodial", "USD", 30000) // $300, excluded by choice
	a2.ExcludeFromNetWorth = true
	accounts := []domain.Account{
		asset("a1", "Checking", "USD", 100000), // $1,000
		a2,
	}
	rates := currency.Rates{Base: "USD"}

	res, err := NetWorthExplained(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetWorthExplained: %v", err)
	}
	if res.Net.Amount != 100000 {
		t.Errorf("net = %d, want 100000 (custodial account excluded by choice)", res.Net.Amount)
	}
	if len(res.ExcludedByChoice) != 1 || res.ExcludedByChoice[0] != "Kid's custodial" {
		t.Errorf("ExcludedByChoice = %v, want [Kid's custodial]", res.ExcludedByChoice)
	}
}
