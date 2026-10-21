// SPDX-License-Identifier: MIT

package ledger

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// AccountNetValueSeries values an explicit set of accounts in base currency at
// each cutoff. Unlike household NetWorthSeries, accounts excluded from household
// net worth remain in this class/portfolio view. Archived accounts are omitted.
// Assets contribute their signed balance; liabilities subtract its checked
// magnitude, including a stored class override. Transactions strictly before
// each cutoff count. Missing rates or malformed balances fail the whole series.
func AccountNetValueSeries(accounts []domain.Account, all []domain.Transaction, cutoffs []time.Time, rates currency.Rates) ([]money.Money, error) {
	return accountValueSeries(accounts, all, cutoffs, rates, accountTotalsPolicy{includeChosenAccounts: true}, "account value")
}

func strictAccountTotals(accounts []domain.Account, all []domain.Transaction, rates currency.Rates, policy accountTotalsPolicy) (NetWorthResult, error) {
	return accumulateAccountTotals(accounts, rates, func(a domain.Account) (money.Money, error) {
		return Balance(a, all)
	}, policy)
}

func accountValueSeries(accounts []domain.Account, all []domain.Transaction, cutoffs []time.Time, rates currency.Rates, policy accountTotalsPolicy, label string) ([]money.Money, error) {
	out := make([]money.Money, len(cutoffs))
	if len(cutoffs) == 0 {
		return out, nil
	}
	// Group row positions without sorting or doing arithmetic. Each account's
	// original transaction order is essential: checked intermediate additions
	// can overflow even when a later opposite-signed row would cancel them.
	rows := accountValueRows(accounts, all, policy)
	for i, c := range cutoffs {
		// Keep account iteration and all errors in the canonical accumulator.
		// Duplicate IDs share positions, never the record's opening balance,
		// currency or computed result.
		res, err := accumulateAccountTotals(accounts, rates, func(a domain.Account) (money.Money, error) {
			return accountValueAt(a, all, rows[a.ID], c)
		}, policy)
		if err != nil {
			return nil, fmt.Errorf("ledger: %s as of %s: %w", label, c.Format(dateutil.Layout), err)
		}
		out[i] = res.Net
	}
	return out, nil
}

func accountValueRows(accounts []domain.Account, all []domain.Transaction, policy accountTotalsPolicy) map[string][]int {
	rows := make(map[string][]int, len(accounts))
	for _, a := range accounts {
		if a.Archived || (!a.CountsInNetWorth() && !policy.includeChosenAccounts) {
			continue
		}
		rows[a.ID] = nil
	}
	if len(rows) == 0 {
		return rows
	}
	for i := range all {
		if positions, ok := rows[all[i].AccountID]; ok {
			rows[all[i].AccountID] = append(positions, i)
		}
	}
	return rows
}

func accountValueAt(a domain.Account, all []domain.Transaction, rows []int, cutoff time.Time) (money.Money, error) {
	bal, err := openingBalance(a)
	if err != nil {
		return money.Money{}, err
	}
	for _, i := range rows {
		txn := &all[i]
		if !txn.Date.Before(cutoff) {
			continue
		}
		bal, err = bal.Add(txn.Amount)
		if err != nil {
			return money.Money{}, fmt.Errorf("ledger: account %s: %w", a.ID, err)
		}
	}
	return bal, nil
}
