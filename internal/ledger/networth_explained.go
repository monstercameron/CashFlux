// SPDX-License-Identifier: MIT

package ledger

import (
	"errors"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// NetWorthResult is net worth with explainability: the base-currency totals plus
// any accounts that could not be converted because their currency has no exchange
// rate. The UI shows the figure AND a notice, rather than silently collapsing the
// whole total to zero (the determinism rule).
type NetWorthResult struct {
	Net               money.Money
	Assets            money.Money
	Liabilities       money.Money
	MissingCurrencies []string // sorted, unique currency codes with no rate
	ExcludedAccounts  []string // names of accounts excluded for a missing rate
	// ExcludedByChoice holds the names of accounts the household deliberately
	// flagged out of net worth (Account.ExcludeFromNetWorth, AC11), in account
	// order. The net-worth surface discloses the count so the figure is never
	// silently reduced — the same contract as ExcludedAccounts above.
	ExcludedByChoice []string
}

// NetWorthExplained is like NetWorth but never fails on a missing exchange rate:
// an account whose currency has no rate is EXCLUDED from the totals and reported
// in MissingCurrencies/ExcludedAccounts — never treated as base or zero. Other
// errors (e.g. a corrupt balance) still propagate.
func NetWorthExplained(accounts []domain.Account, all []domain.Transaction, rates currency.Rates) (NetWorthResult, error) {
	return netWorthAccumulate(accounts, rates, func(a domain.Account) (money.Money, error) {
		return Balance(a, all)
	})
}

// netWorthAccumulate is the shared net-worth core: balanceOf supplies each
// non-archived account's balance (a live scan for NetWorthExplained, a map
// lookup for NetWorthFromBalances), and the accumulation — missing-rate
// exclusion, liability magnitude via Abs, net = assets − liabilities — lives
// in exactly one place.
func netWorthAccumulate(accounts []domain.Account, rates currency.Rates, balanceOf func(domain.Account) (money.Money, error)) (NetWorthResult, error) {
	res := NetWorthResult{Assets: money.Zero(rates.Base), Liabilities: money.Zero(rates.Base)}
	missing := map[string]bool{}
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		// AC11: accounts the household flagged out of net worth are omitted from
		// the totals and disclosed by name, never silently dropped.
		if a.ExcludeFromNetWorth {
			res.ExcludedByChoice = append(res.ExcludedByChoice, a.Name)
			continue
		}
		bal, err := balanceOf(a)
		if err != nil {
			return NetWorthResult{}, err
		}
		conv, err := rates.Convert(bal, rates.Base)
		if err != nil {
			if errors.Is(err, currency.ErrUnknownRate) {
				missing[strings.ToUpper(strings.TrimSpace(a.Currency))] = true
				res.ExcludedAccounts = append(res.ExcludedAccounts, a.Name)
				continue
			}
			return NetWorthResult{}, err
		}
		if a.Class == domain.ClassLiability {
			// A liability's amount owed is the magnitude of its balance, regardless
			// of how the balance is signed at rest: the sample seeds liabilities
			// negative, but an account added through the "amount you owe" form stores
			// it positive. Abs() makes net worth correct for both, rather than
			// silently adding a positive-stored debt to net worth.
			if res.Liabilities, err = res.Liabilities.Add(conv.Abs()); err != nil {
				return NetWorthResult{}, err
			}
		} else {
			if res.Assets, err = res.Assets.Add(conv); err != nil {
				return NetWorthResult{}, err
			}
		}
	}
	net, err := res.Assets.Sub(res.Liabilities)
	if err != nil {
		return NetWorthResult{}, err
	}
	res.Net = net
	for c := range missing {
		res.MissingCurrencies = append(res.MissingCurrencies, c)
	}
	sort.Strings(res.MissingCurrencies)
	return res, nil
}
