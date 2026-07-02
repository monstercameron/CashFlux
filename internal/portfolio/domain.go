// SPDX-License-Identifier: MIT

package portfolio

import "github.com/monstercameron/CashFlux/internal/domain"

// FromDomain converts a persisted domain.Holding to the pure portfolio.Holding
// used by the calculation functions in this package. The domain ID and
// AccountID fields are not needed by the math and are intentionally dropped.
func FromDomain(h domain.Holding) Holding {
	return Holding{
		Ticker:                    h.Ticker,
		Name:                      h.Name,
		Shares:                    h.Shares,
		CostBasisMinor:            h.CostBasisMinor,
		CurrentPriceMinorPerShare: h.CurrentPriceMinorPerShare,
		AssetClass:                h.AssetClass,
		SecurityType:              string(h.SecurityType.Normalized()),
	}
}

// FromDomainSlice converts a slice of domain.Holding values to the pure
// portfolio.Holding slice expected by PortfolioSummary, AllocationByHolding,
// and AllocationByAssetClass.
func FromDomainSlice(hs []domain.Holding) []Holding {
	out := make([]Holding, len(hs))
	for i, h := range hs {
		out[i] = FromDomain(h)
	}
	return out
}
