// SPDX-License-Identifier: MIT

package portfolio_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/portfolio"
)

func TestFromDomainMapsSecurityType(t *testing.T) {
	got := portfolio.FromDomain(domain.Holding{Name: "Apple", SecurityType: domain.SecurityStock})
	if got.SecurityType != "stock" {
		t.Errorf("SecurityType: want stock, got %q", got.SecurityType)
	}
	// Empty normalizes to "other".
	got2 := portfolio.FromDomain(domain.Holding{Name: "Mystery"})
	if got2.SecurityType != "other" {
		t.Errorf("empty SecurityType should normalize to other, got %q", got2.SecurityType)
	}
}

func TestAllocationBySecurityType(t *testing.T) {
	// $6,000 of stock (2 positions) vs $2,000 of bond → 75% / 25%.
	hs := []portfolio.Holding{
		{Name: "A", Shares: 10, CurrentPriceMinorPerShare: 30000, SecurityType: "stock"}, // $3,000
		{Name: "B", Shares: 10, CurrentPriceMinorPerShare: 30000, SecurityType: "stock"}, // $3,000
		{Name: "C", Shares: 10, CurrentPriceMinorPerShare: 20000, SecurityType: "bond"},  // $2,000
	}
	w := portfolio.AllocationBySecurityType(hs)
	if len(w) != 2 {
		t.Fatalf("want 2 groups, got %d: %+v", len(w), w)
	}
	// Sorted by value desc → stock first.
	if w[0].Label != "stock" || w[0].ValueMinor != 600000 {
		t.Errorf("group[0] want stock/600000, got %+v", w[0])
	}
	if w[0].Pct < 74 || w[0].Pct > 76 {
		t.Errorf("stock pct want ~75, got %.1f", w[0].Pct)
	}
	if w[1].Label != "bond" || w[1].ValueMinor != 200000 {
		t.Errorf("group[1] want bond/200000, got %+v", w[1])
	}
	if portfolio.AllocationBySecurityType(nil) != nil {
		t.Error("empty slice should return nil")
	}
}

func TestSecurityTypeValidity(t *testing.T) {
	if !domain.SecurityStock.Valid() || !domain.SecurityType("").Valid() {
		t.Error("stock and empty should be valid")
	}
	if domain.SecurityType("bogus").Valid() {
		t.Error("bogus should be invalid")
	}
	if domain.SecurityType("").Normalized() != domain.SecurityOther {
		t.Error("empty should normalize to other")
	}
}
