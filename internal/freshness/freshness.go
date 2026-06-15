// Package freshness decides when an account balance is likely stale and worth a
// friendly nudge to update. Balances that drift on their own — credit cards,
// loans, lines of credit — go stale sooner than slow-moving savings. Recurring
// fixed bills are modeled as Recurring items (not accounts) and therefore never
// go stale here; a window of 0 also marks a type exempt.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package freshness

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Windows maps an account type to its staleness window in days. A type absent
// from the map, or mapped to a value <= 0, is never considered stale.
type Windows map[domain.AccountType]int

// DefaultWindows returns CashFlux's default staleness windows. Debt-like
// balances drift fastest and so have the shortest windows.
func DefaultWindows() Windows {
	return Windows{
		domain.TypeCreditCard:   14,
		domain.TypeLineOfCredit: 14,
		domain.TypeLoan:         14,
		domain.TypePersonalLoan: 14,
		domain.TypeMortgage:     14,
		domain.TypeChecking:     30,
		domain.TypeDebit:        30,
		domain.TypeCash:         30,
		domain.TypeSavings:      45,
		domain.TypeInvestment:   60,
		domain.TypeOther:        30,
	}
}

// WindowDays returns the staleness window for a type and whether it is tracked.
func (w Windows) WindowDays(t domain.AccountType) (int, bool) {
	d, ok := w[t]
	return d, ok
}

// Merge returns a copy of w with overrides applied (overrides win). Use this to
// layer user settings over the defaults.
func (w Windows) Merge(overrides Windows) Windows {
	out := make(Windows, len(w)+len(overrides))
	for k, v := range w {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

// IsStale reports whether an account's balance is stale as of now. Archived
// accounts and exempt/untracked types are never stale; a tracked account whose
// balance has never been confirmed (zero BalanceAsOf) is treated as stale.
func IsStale(account domain.Account, windows Windows, now time.Time) bool {
	if account.Archived {
		return false
	}
	days, ok := windows.WindowDays(account.Type)
	if !ok || days <= 0 {
		return false
	}
	if account.BalanceAsOf.IsZero() {
		return true
	}
	deadline := account.BalanceAsOf.AddDate(0, 0, days)
	return now.After(deadline)
}

// DaysSinceUpdate returns whole days since the balance was confirmed, or -1 if
// it has never been confirmed.
func DaysSinceUpdate(account domain.Account, now time.Time) int {
	if account.BalanceAsOf.IsZero() {
		return -1
	}
	return dateutil.DaysBetween(account.BalanceAsOf, now)
}

// StaleAccounts returns the accounts whose balances are stale, in input order.
func StaleAccounts(accounts []domain.Account, windows Windows, now time.Time) []domain.Account {
	var out []domain.Account
	for _, a := range accounts {
		if IsStale(a, windows, now) {
			out = append(out, a)
		}
	}
	return out
}
