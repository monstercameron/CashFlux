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
	for i, c := range cutoffs {
		upto := make([]domain.Transaction, 0, len(all))
		for _, txn := range all {
			if txn.Date.Before(c) {
				upto = append(upto, txn)
			}
		}
		res, err := strictAccountTotals(accounts, upto, rates, policy)
		if err != nil {
			return nil, fmt.Errorf("ledger: %s as of %s: %w", label, c.Format(dateutil.Layout), err)
		}
		out[i] = res.Net
	}
	return out, nil
}
