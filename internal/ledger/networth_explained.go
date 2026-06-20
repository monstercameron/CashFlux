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
}

// NetWorthExplained is like NetWorth but never fails on a missing exchange rate:
// an account whose currency has no rate is EXCLUDED from the totals and reported
// in MissingCurrencies/ExcludedAccounts — never treated as base or zero. Other
// errors (e.g. a corrupt balance) still propagate.
func NetWorthExplained(accounts []domain.Account, all []domain.Transaction, rates currency.Rates) (NetWorthResult, error) {
	res := NetWorthResult{Assets: money.Zero(rates.Base), Liabilities: money.Zero(rates.Base)}
	missing := map[string]bool{}
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		bal, err := Balance(a, all)
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
			if res.Liabilities, err = res.Liabilities.Add(conv.Neg()); err != nil {
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
