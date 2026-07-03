// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddNetWorthVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	acct := func(id string, ty domain.AccountType, class domain.AccountClass, minor int64) domain.Account {
		return domain.Account{ID: id, Type: ty, Class: class, Currency: "USD",
			OpeningBalance: money.New(minor, "USD"), BalanceAsOf: asOf}
	}
	d := Data{
		Accounts: []domain.Account{
			acct("a1", domain.TypeChecking, domain.ClassAsset, 100000),    // $1,000 cash
			acct("a2", domain.TypeSavings, domain.ClassAsset, 300000),     // $3,000 cash
			acct("a3", domain.TypeInvestment, domain.ClassAsset, 400000),  // $4,000 invested
			acct("a4", domain.TypeProperty, domain.ClassAsset, 2000000),   // $20,000 property
			acct("a5", domain.TypeOther, domain.ClassAsset, 200000),       // $2,000 other
			acct("l1", domain.TypeCreditCard, domain.ClassLiability, -50000), // liability — excluded from buckets
		},
		Transactions: []domain.Transaction{
			// July deposit so the month-to-date change is non-zero: +$500 into
			// checking on Jul 10 → NW now sits $500 above the July start.
			{ID: "t1", AccountID: "a1", Date: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(50000, "USD")},
		},
		Rates: currency.Rates{Base: "USD"}, Now: now,
	}
	v := Vars(d)

	if got := v["networth_cash"]; got != 4500 { // 1,000 + 3,000 + 500 deposit
		t.Errorf("networth_cash = %v, want 4500", got)
	}
	if got := v["networth_invested"]; got != 4000 {
		t.Errorf("networth_invested = %v, want 4000", got)
	}
	if got := v["networth_property"]; got != 20000 {
		t.Errorf("networth_property = %v, want 20000", got)
	}
	if got := v["networth_other_assets"]; got != 2000 {
		t.Errorf("networth_other_assets = %v, want 2000", got)
	}
	// Liquid share: 4,500 of 30,500 total assets ≈ 14.75%.
	if got := v["networth_liquid_pct"]; got < 14 || got > 16 {
		t.Errorf("networth_liquid_pct = %v, want ~14.75", got)
	}
	// Change: the June deposit lifts July-start NW by $500 over June-start.
	if got := v["networth_change"]; got != 500 {
		t.Errorf("networth_change = %v, want 500", got)
	}
	if got := v["networth_change_pct"]; got <= 0 || got > 3 {
		t.Errorf("networth_change_pct = %v, want a small positive percent", got)
	}
}

// TestAddNetWorthVarsEmpty asserts every networth_* variable is always present
// so formulas referencing them never hit undefined-variable errors.
func TestAddNetWorthVarsEmpty(t *testing.T) {
	v := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	for _, k := range NetWorthVarNames {
		if _, ok := v[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
}
