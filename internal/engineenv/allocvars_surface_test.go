// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddAllocVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accts := []domain.Account{
		{ID: "a1", Name: "Brokerage", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(100000)},
		{ID: "a2", Name: "Savings", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: usd(100000)},
		{ID: "d1", Name: "Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", InterestRateAPR: 22},
		{ID: "d2", Name: "0% Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", InterestRateAPR: 0},
		{ID: "x", Name: "Archived", Class: domain.ClassAsset, Currency: "USD", Archived: true},
	}
	goals := []domain.Goal{{ID: "g1", Name: "Trip"}, {ID: "g2", Name: "Car"}}
	alloc := AllocData{AmountMinor: 200000, ReserveMinor: 50000, MaxPerMinor: 60000} // $2000 / $500 / $600

	vars := Vars(Data{Accounts: accts, Goals: goals, Alloc: alloc, Rates: currency.Rates{Base: "USD"}, Now: now})

	if got := vars["alloc_amount"]; got != 2000 {
		t.Errorf("alloc_amount = %v, want 2000", got)
	}
	if got := vars["alloc_reserve"]; got != 500 {
		t.Errorf("alloc_reserve = %v, want 500", got)
	}
	if got := vars["alloc_max_per"]; got != 600 {
		t.Errorf("alloc_max_per = %v, want 600", got)
	}
	// allocatable = amount - reserve = $1500.
	if got := vars["alloc_allocatable"]; got != 1500 {
		t.Errorf("alloc_allocatable = %v, want 1500", got)
	}
	// reserved% = 500/2000 = 25%.
	if got := vars["alloc_reserved_pct"]; got != 25 {
		t.Errorf("alloc_reserved_pct = %v, want 25", got)
	}
	// destinations: 2 asset accounts + 1 interest-bearing debt (the 0% card and archived
	// account don't count) + 2 goals = 5.
	if got := vars["alloc_destination_count"]; got != 5 {
		t.Errorf("alloc_destination_count = %v, want 5", got)
	}
}

func TestAddAllocVarsEmptyPlan(t *testing.T) {
	vars := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	// An empty plan floors cleanly at zero (no divide-by-zero on reserved%).
	for _, k := range []string{"alloc_amount", "alloc_reserve", "alloc_allocatable", "alloc_reserved_pct", "alloc_destination_count"} {
		if got := vars[k]; got != 0 {
			t.Errorf("%s = %v, want 0 on an empty plan", k, got)
		}
	}
}
