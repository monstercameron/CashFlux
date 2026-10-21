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

func TestAddNetWorthVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	acct := func(id string, ty domain.AccountType, class domain.AccountClass, minor int64) domain.Account {
		return domain.Account{ID: id, Type: ty, Class: class, Currency: "USD",
			OpeningBalance: money.New(minor, "USD"), BalanceAsOf: asOf}
	}
	d := Data{
		Accounts: []domain.Account{
			acct("a1", domain.TypeChecking, domain.ClassAsset, 100000),       // $1,000 cash
			acct("a2", domain.TypeSavings, domain.ClassAsset, 300000),        // $3,000 cash
			acct("a3", domain.TypeInvestment, domain.ClassAsset, 400000),     // $4,000 invested
			acct("a4", domain.TypeProperty, domain.ClassAsset, 2000000),      // $20,000 property
			acct("a5", domain.TypeOther, domain.ClassAsset, 200000),          // $2,000 other
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

// Empty asset buckets are known zeroes. Change and liquid percentage require
// observations and stay unavailable when no balances are present.
func TestAddNetWorthVarsEmpty(t *testing.T) {
	v := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	for _, k := range []string{"networth_cash", "networth_invested", "networth_property", "networth_other_assets"} {
		if got, ok := v[k]; !ok || got != 0 {
			t.Errorf("empty bucket %s = %v, present %v; want known zero", k, got, ok)
		}
	}
	if got, ok := v["networth_change"]; !ok || got != 0 {
		t.Errorf("empty net-worth change = %v, present %v; want known zero", got, ok)
	}
	for _, k := range []string{"networth_change_pct", "networth_liquid_pct"} {
		if _, ok := v[k]; ok {
			t.Errorf("%s should be unavailable without observations", k)
		}
	}
}

func TestNetWorthCompositionOmitsOnlyBucketsWithUnknownFX(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	vars := Vars(Data{
		Accounts: []domain.Account{
			{ID: "cash-eur", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR", OpeningBalance: money.New(10000, "EUR")},
			{ID: "invest-usd", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: money.New(25000, "USD")},
		},
		Rates: currency.Rates{Base: "USD"}, Now: now,
		Molecules: []domain.Molecule{
			{Name: "networth_cash", Formula: "999"},
			{Name: "networth_liquid_pct", Formula: "999"},
		},
	})
	for _, name := range []string{"networth_cash", "networth_liquid_pct"} {
		if _, ok := vars[name]; ok {
			t.Errorf("unavailable %s was supplied by a molecule", name)
		}
	}
	if got, ok := vars["networth_invested"]; !ok || got != 250 {
		t.Fatalf("independent USD bucket = %v, present %v; want 250", got, ok)
	}
	if _, ok := vars["networth_liquid_pct"]; ok {
		t.Fatal("liquid percentage should be unavailable when an asset is unconvertible")
	}
}

func TestNetWorthCompositionOverflowOmitsAffectedTotals(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	vars := Vars(Data{
		Accounts: []domain.Account{
			{ID: "large-cash", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")},
			{ID: "extra-cash", Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: money.New(1, "USD")},
			{ID: "investment", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: money.New(500, "USD")},
		},
		Rates: currency.Rates{Base: "USD"}, Now: now,
	})
	if _, ok := vars["networth_cash"]; ok {
		t.Fatal("overflowing cash bucket should be unavailable")
	}
	if got, ok := vars["networth_invested"]; !ok || got != 5 {
		t.Fatalf("independent investment bucket = %v, present %v; want 5", got, ok)
	}
	if _, ok := vars["networth_liquid_pct"]; ok {
		t.Fatal("liquid percentage should be unavailable when asset total overflows")
	}
}

func TestNetWorthChangeOmitsUnrepresentableDelta(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	posted := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	vars := Vars(Data{
		Accounts: []domain.Account{{
			ID: "cash", Class: domain.ClassAsset, Type: domain.TypeChecking,
			Currency: "USD", OpeningBalance: money.New(-math.MaxInt64, "USD"),
		}},
		Transactions: []domain.Transaction{
			{ID: "in-one", AccountID: "cash", Date: posted, Amount: money.New(math.MaxInt64, "USD")},
			{ID: "in-two", AccountID: "cash", Date: posted.Add(time.Hour), Amount: money.New(math.MaxInt64, "USD")},
		},
		Rates: currency.Rates{Base: "USD"}, Now: now,
	})
	for _, name := range []string{"networth_change", "networth_change_pct"} {
		if _, ok := vars[name]; ok {
			t.Errorf("overflowing %s should be unavailable", name)
		}
	}
	if got, ok := vars["networth_cash"]; !ok || got != float64(math.MaxInt64)/100 {
		t.Fatalf("current cash balance = %v, present %v; want known current balance", got, ok)
	}
}
