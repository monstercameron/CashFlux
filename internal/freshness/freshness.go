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
		domain.TypeInvestment:   120,
		domain.TypeRetirement:   120, // C73: slow-moving like investments — same long window
		domain.TypeCrypto:       30,  // C73: volatile but manually updated — monthly cadence
		domain.TypeProperty:     180, // C224: illiquid real-estate valuation — same long window as Other
		domain.TypeVehicle:      180, // C224: illiquid vehicle valuation — same long window as Other
		domain.TypeOther:        180,
	}
}

// WindowDays returns the staleness window for a type and whether it is tracked.
func (w Windows) WindowDays(t domain.AccountType) (int, bool) {
	d, ok := w[t]
	return d, ok
}

// EffectiveWindowDays returns the staleness/revaluation window for a specific
// account, honouring its per-account override (Account.RevalueDays, AC5). A
// positive RevalueDays always wins over the type default — this lets a single
// property or vehicle be re-estimated on its own schedule without a nag on the
// shared clock. Zero RevalueDays falls back to the type window.
func (w Windows) EffectiveWindowDays(a domain.Account) (int, bool) {
	if a.RevalueDays > 0 {
		return a.RevalueDays, true
	}
	return w.WindowDays(a.Type)
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
// accounts, freshness-exempt accounts, accounts snoozed past now, and
// exempt/untracked types are never stale; a tracked account whose balance has
// never been confirmed (zero BalanceAsOf) is treated as stale.
func IsStale(account domain.Account, windows Windows, now time.Time) bool {
	if account.Archived || account.FreshnessExempt {
		return false
	}
	if !account.FreshnessSnoozeUntil.IsZero() && now.Before(account.FreshnessSnoozeUntil) {
		return false
	}
	days, ok := windows.EffectiveWindowDays(account)
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

// Standing is the four-state label for how much of a page can be trusted
// (EC-20, and the WF4 refinement's "Current / Mostly current / Incomplete /
// Stale").
//
// It exists because a page of figures derived from balances nobody has confirmed
// in two months looks exactly like a page derived from balances confirmed this
// morning. The numbers are equally crisp and one of them is fiction, and the app
// is the only party in a position to say which.
type Standing string

const (
	// StandingCurrent: everything behind this page was confirmed recently.
	StandingCurrent Standing = "current"
	// StandingMostly: most of it was, and what is stale is named.
	StandingMostly Standing = "mostly"
	// StandingIncomplete: a substantial share is stale or was never confirmed.
	StandingIncomplete Standing = "incomplete"
	// StandingStale: most of it is out of date. The figures still compute; they
	// just are not evidence of anything.
	StandingStale Standing = "stale"
	// StandingUnknown: there is nothing to judge — no accounts at all. Distinct
	// from Current, because "nothing is stale" and "there is nothing" are
	// different statements and only the first is reassuring.
	StandingUnknown Standing = "unknown"
)

// MostlyShare and IncompleteShare are where the four states divide.
//
// Two thirds and one third. A single stale account out of ten should not drop a
// page from "current" to an alarm — it should say "mostly, and here is which" —
// while a page where two accounts in three are unconfirmed has to stop implying
// its totals are facts.
const (
	MostlyShare     = 2.0 / 3.0
	IncompleteShare = 1.0 / 3.0
)

// Coverage is how much of a page's underlying data is confirmed.
type Coverage struct {
	Total int
	// Fresh is confirmed within its account's window.
	Fresh int
	// Stale is confirmed, but too long ago.
	Stale int
	// Unconfirmed is an account whose balance has never been confirmed at all.
	// Counted apart from Stale because "we have not checked since March" and "we
	// have never checked" call for different actions.
	Unconfirmed int
	// Reconciled is how many carry a recorded statement reconciliation — the
	// strongest evidence an account balance has, and the one figure here that
	// says somebody matched it against the bank rather than just retyping it.
	Reconciled int
}

// Standing is the four-state verdict for this coverage.
func (c Coverage) Standing() Standing {
	if c.Total == 0 {
		return StandingUnknown
	}
	share := float64(c.Fresh) / float64(c.Total)
	switch {
	case c.Fresh == c.Total:
		return StandingCurrent
	case share >= MostlyShare:
		return StandingMostly
	case share >= IncompleteShare:
		return StandingIncomplete
	}
	return StandingStale
}

// Trustworthy reports whether a page's figures rest on mostly-confirmed data.
// Callers that want to caveat a headline read this; callers that want to explain
// read the counts.
func (c Coverage) Trustworthy() bool {
	s := c.Standing()
	return s == StandingCurrent || s == StandingMostly
}

// Measure summarizes how much of a set of accounts is confirmed.
//
// Archived accounts are excluded: they are not part of any page's figures, and
// counting them would drag every household's standing down for accounts they
// deliberately closed.
func Measure(accounts []domain.Account, windows Windows, now time.Time) Coverage {
	var c Coverage
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		c.Total++
		if len(a.Reconciliations) > 0 {
			c.Reconciled++
		}
		if a.BalanceAsOf.IsZero() {
			c.Unconfirmed++
			continue
		}
		if IsStale(a, windows, now) {
			c.Stale++
			continue
		}
		c.Fresh++
	}
	return c
}
