// SPDX-License-Identifier: MIT

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

// Dismissals records when a stale-account nudge was dismissed, keyed by account
// ID. A dismissal only applies while the account's BalanceAsOf is older than the
// dismissal; once the balance is marked updated, future stale nudges can return.
type Dismissals map[string]time.Time

// DefaultWindows returns CashFlux's default staleness windows. Debt-like
// balances drift fastest and so have the shortest windows; slow-moving asset
// values get much longer ones. C222/C226: an investment or other-asset balance
// (the bucket today's property/vehicle valuations land in) is a periodically
// estimated figure, not a reconciled cash balance, so nagging it monthly is
// wrong — investments go 120 days and other illiquid assets 180 before a nudge.
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
		domain.TypeInvestment:  120,
		domain.TypeRetirement:  120, // C73: slow-moving like investments — same long window
		domain.TypeCrypto:       30, // C73: volatile but manually updated — monthly cadence
		domain.TypeProperty:    180, // C224: illiquid real-estate valuation — same long window as Other
		domain.TypeVehicle:     180, // C224: illiquid vehicle valuation — same long window as Other
		domain.TypeOther:        180,
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

// Dismiss marks the given accounts dismissed as of at, returning a copy so UI
// state can update immutably.
func (d Dismissals) Dismiss(accounts []domain.Account, at time.Time) Dismissals {
	out := make(Dismissals, len(d)+len(accounts))
	for id, dismissedAt := range d {
		out[id] = dismissedAt
	}
	for _, a := range accounts {
		if a.ID != "" {
			out[a.ID] = at
		}
	}
	return out
}

// IsDismissed reports whether account's current stale state has been dismissed.
func (d Dismissals) IsDismissed(account domain.Account) bool {
	dismissedAt, ok := d[account.ID]
	if !ok {
		return false
	}
	return account.BalanceAsOf.IsZero() || !dismissedAt.Before(account.BalanceAsOf)
}

// VisibleStaleAccounts returns stale accounts whose current stale state has not
// been dismissed.
func VisibleStaleAccounts(accounts []domain.Account, windows Windows, dismissals Dismissals, now time.Time) []domain.Account {
	stale := StaleAccounts(accounts, windows, now)
	out := make([]domain.Account, 0, len(stale))
	for _, a := range stale {
		if !dismissals.IsDismissed(a) {
			out = append(out, a)
		}
	}
	return out
}
