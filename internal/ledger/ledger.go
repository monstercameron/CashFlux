// Package ledger derives balances and totals from accounts and transactions:
// account balances, running balances, period income/expense, net worth, and
// per-owner rollups. Cross-currency aggregates convert to the base currency of
// the supplied rate table.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package ledger

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// openingBalance returns the account's opening balance normalized to its
// currency, or an error if a non-empty opening balance uses a different currency.
func openingBalance(account domain.Account) (money.Money, error) {
	bal := account.OpeningBalance
	if bal.Currency == "" {
		return money.Zero(account.Currency), nil
	}
	if bal.Currency != account.Currency {
		return money.Money{}, fmt.Errorf("ledger: account %s opening balance currency %q != account currency %q", account.ID, bal.Currency, account.Currency)
	}
	return bal, nil
}

// Balance returns an account's opening balance plus all of its transactions.
// Transactions for other accounts are ignored. All amounts must be in the
// account's currency.
func Balance(account domain.Account, all []domain.Transaction) (money.Money, error) {
	bal, err := openingBalance(account)
	if err != nil {
		return money.Money{}, err
	}
	for _, t := range all {
		if t.AccountID != account.ID {
			continue
		}
		next, err := bal.Add(t.Amount)
		if err != nil {
			return money.Money{}, fmt.Errorf("ledger: account %s: %w", account.ID, err)
		}
		bal = next
	}
	return bal, nil
}

// ClearedBalance returns an account's opening balance plus only its cleared
// transactions — the figure to reconcile against a statement. Uncleared
// transactions are excluded.
func ClearedBalance(account domain.Account, all []domain.Transaction) (money.Money, error) {
	bal, err := openingBalance(account)
	if err != nil {
		return money.Money{}, err
	}
	for _, t := range all {
		if t.AccountID != account.ID || !t.Cleared {
			continue
		}
		next, err := bal.Add(t.Amount)
		if err != nil {
			return money.Money{}, fmt.Errorf("ledger: account %s: %w", account.ID, err)
		}
		bal = next
	}
	return bal, nil
}

// RunningBalances returns the cumulative balance after each of the account's
// transactions, in the order given. Sort by date beforehand for a chronological
// series.
func RunningBalances(account domain.Account, ordered []domain.Transaction) ([]money.Money, error) {
	bal, err := openingBalance(account)
	if err != nil {
		return nil, err
	}
	var out []money.Money
	for _, t := range ordered {
		if t.AccountID != account.ID {
			continue
		}
		next, err := bal.Add(t.Amount)
		if err != nil {
			return nil, fmt.Errorf("ledger: account %s: %w", account.ID, err)
		}
		bal = next
		out = append(out, bal)
	}
	return out, nil
}

// PeriodTotals sums non-transfer income and expense within the half-open range
// [start, end), converting each transaction to the base currency. Both returned
// values are non-negative and in the base currency.
func PeriodTotals(all []domain.Transaction, start, end time.Time, rates currency.Rates) (income, expense money.Money, err error) {
	income = money.Zero(rates.Base)
	expense = money.Zero(rates.Base)
	for _, t := range all {
		if t.IsTransfer() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return money.Money{}, money.Money{}, err
		}
		switch {
		case t.IsIncome():
			if income, err = income.Add(conv); err != nil {
				return money.Money{}, money.Money{}, err
			}
		case t.IsExpense():
			if expense, err = expense.Add(conv.Abs()); err != nil {
				return money.Money{}, money.Money{}, err
			}
		}
	}
	return income, expense, nil
}

// NetWorth returns net worth (assets − liabilities) along with the asset and
// liability totals, all in the base currency. Archived accounts are excluded.
// Liability amounts are reported as positive amounts owed.
func NetWorth(accounts []domain.Account, all []domain.Transaction, rates currency.Rates) (net, assets, liabilities money.Money, err error) {
	assets = money.Zero(rates.Base)
	liabilities = money.Zero(rates.Base)
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		bal, err := Balance(a, all)
		if err != nil {
			return money.Money{}, money.Money{}, money.Money{}, err
		}
		conv, err := rates.Convert(bal, rates.Base)
		if err != nil {
			return money.Money{}, money.Money{}, money.Money{}, err
		}
		if a.Class == domain.ClassLiability {
			if liabilities, err = liabilities.Add(conv.Neg()); err != nil {
				return money.Money{}, money.Money{}, money.Money{}, err
			}
		} else {
			if assets, err = assets.Add(conv); err != nil {
				return money.Money{}, money.Money{}, money.Money{}, err
			}
		}
	}
	net, err = assets.Sub(liabilities)
	if err != nil {
		return money.Money{}, money.Money{}, money.Money{}, err
	}
	return net, assets, liabilities, nil
}

// NetWorthSeries returns net worth as of each cutoff time, all in the base
// currency. Transactions strictly before a cutoff are counted, so passing the
// first day of successive months yields an end-of-month net-worth trend.
// Archived accounts are excluded, as in NetWorth.
func NetWorthSeries(accounts []domain.Account, all []domain.Transaction, cutoffs []time.Time, rates currency.Rates) ([]money.Money, error) {
	out := make([]money.Money, len(cutoffs))
	for i, c := range cutoffs {
		upto := make([]domain.Transaction, 0, len(all))
		for _, t := range all {
			if t.Date.Before(c) {
				upto = append(upto, t)
			}
		}
		net, _, _, err := NetWorth(accounts, upto, rates)
		if err != nil {
			return nil, fmt.Errorf("ledger: net worth as of %s: %w", c.Format(dateutil.Layout), err)
		}
		out[i] = net
	}
	return out, nil
}

// NetByOwner returns each owner's net worth (sum of their account balances in
// base currency) keyed by owner ID — member IDs plus domain.GroupOwnerID for
// shared accounts. Archived accounts are excluded.
func NetByOwner(accounts []domain.Account, all []domain.Transaction, rates currency.Rates) (map[string]money.Money, error) {
	out := make(map[string]money.Money)
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		bal, err := Balance(a, all)
		if err != nil {
			return nil, err
		}
		conv, err := rates.Convert(bal, rates.Base)
		if err != nil {
			return nil, err
		}
		cur, ok := out[a.OwnerID]
		if !ok {
			cur = money.Zero(rates.Base)
		}
		if cur, err = cur.Add(conv); err != nil {
			return nil, err
		}
		out[a.OwnerID] = cur
	}
	return out, nil
}

// PercentChange returns the whole-percent change from prev to curr (both in the
// same minor units), with ok=false when prev is zero (no meaningful baseline).
// It divides by the magnitude of prev so the sign always reflects the real
// direction even when the baseline is negative — e.g. a net worth moving from
// -100 to -50 is a +50% improvement, not a decline. The result truncates toward
// zero, matching integer division.
func PercentChange(curr, prev int64) (pct int64, ok bool) {
	if prev == 0 {
		return 0, false
	}
	mag := prev
	if mag < 0 {
		mag = -mag
	}
	return (curr - prev) * 100 / mag, true
}
