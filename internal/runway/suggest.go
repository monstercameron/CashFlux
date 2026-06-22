package runway

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// CoverSuggestion is the result of SuggestCover: the best liquid account to
// pull from to cover a cash-flow shortfall, the amount to move (capped at the
// account's available balance), and whether a suitable source was found at all.
// All amounts are integer minor units in the base currency.
type CoverSuggestion struct {
	// Found is true when at least one liquid account in the same currency (after
	// conversion to base) has a positive balance that can contribute toward the
	// shortfall.
	Found bool

	// SourceName is the display name of the recommended source account — the
	// liquid asset account with the largest available balance.
	SourceName string

	// SourceID is the ID of the recommended source account.
	SourceID string

	// AmountMinor is the amount (in base-currency minor units) to transfer from
	// the source — the lesser of the full shortfall and the source's balance.
	AmountMinor int64
}

// isLiquidAsset reports whether an account is a non-archived liquid-cash asset
// (checking, debit, savings, or cash) — the same set used by ledger.LiquidBalance.
func isLiquidAsset(a domain.Account) bool {
	if a.Archived || a.Class == domain.ClassLiability {
		return false
	}
	switch a.Type {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		return true
	default:
		return false
	}
}

// SuggestCover identifies the best liquid account to pull from when the daily
// cash-flow projection detects a shortfall of shortfallMinor base-currency
// minor units. It scans accounts, computes each liquid asset's current balance
// (via ledger.Balance, converted to base), and returns the one with the highest
// positive balance as the recommended source.
//
// The suggestion is currency-aware: only accounts whose balance converts
// successfully to the base currency are considered. The returned AmountMinor is
// min(shortfallMinor, sourceBalance) — never more than the source holds.
//
// A zero or negative shortfall yields CoverSuggestion{Found: false}.
func SuggestCover(
	shortfallMinor int64,
	accounts []domain.Account,
	txns []domain.Transaction,
	rates currency.Rates,
) CoverSuggestion {
	if shortfallMinor <= 0 {
		return CoverSuggestion{}
	}

	var bestName, bestID string
	var bestBal int64

	for _, a := range accounts {
		if !isLiquidAsset(a) {
			continue
		}
		raw, err := ledger.Balance(a, txns)
		if err != nil {
			continue
		}
		if raw.Amount <= 0 {
			continue
		}
		conv, err := rates.Convert(raw, rates.Base)
		if err != nil {
			continue
		}
		if conv.Amount <= 0 {
			continue
		}
		if conv.Amount > bestBal {
			bestBal = conv.Amount
			bestName = a.Name
			bestID = a.ID
		}
	}

	if bestBal <= 0 {
		return CoverSuggestion{}
	}

	move := shortfallMinor
	if bestBal < move {
		move = bestBal
	}
	return CoverSuggestion{Found: true, SourceName: bestName, SourceID: bestID, AmountMinor: move}
}
