// SPDX-License-Identifier: MIT

// Package networthview preserves the distinction between a complete net-worth
// total, a disclosed missing-FX subtotal, and an unavailable calculation.
package networthview

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Snapshot contains immutable display facts. Known makes the monetary values
// usable; Partial requires disclosing MissingCurrencies and ExcludedAccounts.
// Err describes a fatal balance/conversion/arithmetic failure, never a real zero.
type Snapshot struct {
	ledger.NetWorthResult
	Known   bool
	Partial bool
	Err     error
}

// Build delegates the ledger calculation once and retains its error contract.
func Build(accounts []domain.Account, transactions []domain.Transaction, rates currency.Rates) Snapshot {
	result, err := ledger.NetWorthExplained(accounts, transactions, rates)
	return snapshot(result, err)
}

// FromBalances reuses an existing exact account-balance index. It does not fold
// the ledger again; unavailable eligible balances remain fatal to the total.
func FromBalances(accounts []domain.Account, balances map[string]money.Money, rates currency.Rates) Snapshot {
	result, err := ledger.NetWorthFromBalances(accounts, balances, rates)
	return snapshot(result, err)
}

func snapshot(result ledger.NetWorthResult, err error) Snapshot {
	if err != nil {
		return Snapshot{Err: err}
	}
	return Snapshot{NetWorthResult: result, Known: true, Partial: len(result.MissingCurrencies) != 0}
}

// Complete reports whether this is a known total without missing-rate omissions.
// Deliberate account exclusions remain part of the household's stated definition.
func (s Snapshot) Complete() bool { return s.Known && !s.Partial }
