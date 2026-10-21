// SPDX-License-Identifier: MIT

package runway

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// SuggestCoverChecked returns the same largest-positive-liquid-account choice
// as SuggestCover while keeping balance and currency conversion failures visible.
func SuggestCoverChecked(shortfallMinor int64, accounts []domain.Account, txns []domain.Transaction, rates currency.Rates) (CoverSuggestion, error) {
	if shortfallMinor <= 0 {
		return CoverSuggestion{}, nil
	}
	var best CoverSuggestion
	var bestBalance int64
	for _, account := range accounts {
		if !isLiquidAsset(account) {
			continue
		}
		balance, err := ledger.Balance(account, txns)
		if err != nil {
			return CoverSuggestion{}, fmt.Errorf("runway: cover source %q balance: %w", account.ID, err)
		}
		converted, err := rates.Convert(balance, rates.Base)
		if err != nil {
			return CoverSuggestion{}, fmt.Errorf("runway: cover source %q currency: %w", account.ID, err)
		}
		if converted.Amount > bestBalance {
			bestBalance = converted.Amount
			best = CoverSuggestion{Found: true, SourceName: account.Name, SourceID: account.ID}
		}
	}
	if !best.Found {
		return CoverSuggestion{}, nil
	}
	best.AmountMinor = shortfallMinor
	if bestBalance < best.AmountMinor {
		best.AmountMinor = bestBalance
	}
	return best, nil
}
