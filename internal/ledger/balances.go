// SPDX-License-Identifier: MIT

package ledger

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Balances computes every account's balance in ONE pass over the transactions,
// returning a map keyed by account ID. Semantically identical to calling
// Balance once per account — opening balance plus the account's transactions,
// in the account's native currency — but O(A + T) instead of O(A × T), which
// matters when a variable surface needs many balances over a large ledger.
// An account whose balance errors (corrupt opening balance, currency mismatch)
// is absent from the map; the first such error is returned alongside the map,
// which still holds every account that computed cleanly.
func Balances(accounts []domain.Account, all []domain.Transaction) (map[string]money.Money, error) {
	return bulkBalances(accounts, all, false)
}

// ClearedBalances is Balances over only cleared transactions — the bulk
// counterpart of ClearedBalance.
func ClearedBalances(accounts []domain.Account, all []domain.Transaction) (map[string]money.Money, error) {
	return bulkBalances(accounts, all, true)
}

func bulkBalances(accounts []domain.Account, all []domain.Transaction, clearedOnly bool) (map[string]money.Money, error) {
	out := make(map[string]money.Money, len(accounts))
	var firstErr error
	for _, a := range accounts {
		bal, err := openingBalance(a)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out[a.ID] = bal
	}
	for _, t := range all {
		if clearedOnly && !t.Cleared {
			continue
		}
		bal, ok := out[t.AccountID]
		if !ok {
			continue // unknown account, or one already dropped for an error
		}
		next, err := bal.Add(t.Amount)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("ledger: account %s: %w", t.AccountID, err)
			}
			delete(out, t.AccountID)
			continue
		}
		out[t.AccountID] = next
	}
	return out, firstErr
}

// NetWorthFromBalances is NetWorthExplained over precomputed balances (from
// Balances), so a caller that already paid for the single-pass map doesn't
// re-scan the ledger. Same contract: a missing exchange rate excludes the
// account gracefully; any other problem (here: an account absent from the map,
// i.e. its balance errored) fails the result.
func NetWorthFromBalances(accounts []domain.Account, balances map[string]money.Money, rates currency.Rates) (NetWorthResult, error) {
	return netWorthAccumulate(accounts, rates, func(a domain.Account) (money.Money, error) {
		bal, ok := balances[a.ID]
		if !ok {
			return money.Money{}, fmt.Errorf("ledger: account %s: no computed balance", a.ID)
		}
		return bal, nil
	})
}

// LiquidFromBalances is LiquidBalance over precomputed balances (from
// Balances): spendable cash across non-archived cash-type accounts, converted
// to base. Errors mirror LiquidBalance's — a cash account whose balance is
// unavailable or unconvertible fails the total.
func LiquidFromBalances(accounts []domain.Account, balances map[string]money.Money, rates currency.Rates) (money.Money, error) {
	total := money.Zero(rates.Base)
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		switch a.Type {
		case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		default:
			continue
		}
		bal, ok := balances[a.ID]
		if !ok {
			return money.Money{}, fmt.Errorf("ledger: account %s: no computed balance", a.ID)
		}
		conv, err := rates.Convert(bal, rates.Base)
		if err != nil {
			return money.Money{}, err
		}
		if total, err = total.Add(conv); err != nil {
			return money.Money{}, err
		}
	}
	return total, nil
}
