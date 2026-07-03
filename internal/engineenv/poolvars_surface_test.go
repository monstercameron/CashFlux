// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddPoolVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accts := []domain.Account{
		{ID: "a1", Name: "401k", Class: domain.ClassAsset, Type: domain.TypeRetirement, Currency: "USD", OpeningBalance: usd(2000000)},
		{ID: "a2", Name: "Roth", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(800000)},
		{ID: "a3", Name: "Other", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: usd(500000)},
	}
	pools := []PoolDef{{Name: "Retirement", AccountIDs: []string{"a1", "a2"}}}
	vars := Vars(Data{Accounts: accts, Pools: pools, Rates: currency.Rates{Base: "USD"}, Now: now})

	// pool_retirement_value = 401k + Roth = $28,000 (major units).
	if got := vars["pool_retirement_value"]; got != 28000 {
		t.Errorf("pool_retirement_value = %v, want 28000", got)
	}
	// A3 is not in the pool, so it doesn't contribute.
	if _, ok := vars["pool_other_value"]; ok {
		t.Error("no pool named 'Other' — should not appear")
	}
}

func TestPoolVarBasesCollision(t *testing.T) {
	bases := PoolVarBases([]PoolDef{{Name: "Growth"}, {Name: "Growth"}})
	if len(bases) != 2 || bases[0].Prefix != "pool_growth_" || bases[1].Prefix != "pool_growth_2_" {
		t.Errorf("collision handling wrong: %+v", bases)
	}
}
