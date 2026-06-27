// SPDX-License-Identifier: MIT

package portfolio

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestFromDomain(t *testing.T) {
	dh := domain.Holding{
		ID:                        "ignored-id",
		AccountID:                 "ignored-account",
		Ticker:                    "MSFT",
		Name:                      "Microsoft Corporation",
		Shares:                    5.0,
		CostBasisMinor:            150000,
		CurrentPriceMinorPerShare: 40000,
		AssetClass:                "Stocks",
	}

	ph := FromDomain(dh)
	if ph.Ticker != "MSFT" {
		t.Errorf("Ticker = %q, want MSFT", ph.Ticker)
	}
	if ph.Name != "Microsoft Corporation" {
		t.Errorf("Name = %q, want Microsoft Corporation", ph.Name)
	}
	if ph.Shares != 5.0 {
		t.Errorf("Shares = %v, want 5.0", ph.Shares)
	}
	if ph.CostBasisMinor != 150000 {
		t.Errorf("CostBasisMinor = %d, want 150000", ph.CostBasisMinor)
	}
	if ph.CurrentPriceMinorPerShare != 40000 {
		t.Errorf("CurrentPriceMinorPerShare = %d, want 40000", ph.CurrentPriceMinorPerShare)
	}
	if ph.AssetClass != "Stocks" {
		t.Errorf("AssetClass = %q, want Stocks", ph.AssetClass)
	}
}

func TestFromDomainSlicePortfolioSummary(t *testing.T) {
	domainHoldings := []domain.Holding{
		{
			ID: "h1", Ticker: "AAPL", Name: "Apple Inc.",
			Shares: 10, CostBasisMinor: 100000, CurrentPriceMinorPerShare: 15000,
			AssetClass: "Stocks",
		},
		{
			ID: "h2", Ticker: "BND", Name: "Vanguard Bond ETF",
			Shares: 20, CostBasisMinor: 180000, CurrentPriceMinorPerShare: 8000,
			AssetClass: "Bonds",
		},
	}

	phs := FromDomainSlice(domainHoldings)
	if len(phs) != 2 {
		t.Fatalf("FromDomainSlice: got %d, want 2", len(phs))
	}

	// AAPL: value = 10 * 15000 = 150000; BND: value = 20 * 8000 = 160000
	// total cost = 280000; total value = 310000; gain = 30000
	sum := PortfolioSummary(phs)
	if sum.TotalValueMinor != 310000 {
		t.Errorf("TotalValueMinor = %d, want 310000", sum.TotalValueMinor)
	}
	if sum.TotalCostMinor != 280000 {
		t.Errorf("TotalCostMinor = %d, want 280000", sum.TotalCostMinor)
	}
	if sum.TotalGainMinor != 30000 {
		t.Errorf("TotalGainMinor = %d, want 30000", sum.TotalGainMinor)
	}

	// Allocation by asset class should return Stocks and Bonds
	byClass := AllocationByAssetClass(phs)
	if len(byClass) != 2 {
		t.Errorf("AllocationByAssetClass: got %d classes, want 2", len(byClass))
	}
}
