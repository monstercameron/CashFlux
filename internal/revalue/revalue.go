// SPDX-License-Identifier: MIT

// Package revalue supplies revaluation cadences for manual-asset accounts (AC5).
// Property, vehicles, and crypto are periodically ESTIMATED, not reconciled from a
// transaction stream, so they should not share a cash account's staleness clock:
// nagging to re-estimate a house monthly is wrong. This package feeds the existing
// internal/freshness machinery a per-type cadence (property quarterly, vehicle
// semi-annual, crypto weekly) plus a per-account override (domain.Account.RevalueDays),
// so "stale" for a revaluable asset means "time for a fresh estimate" rather than
// "you forgot to enter transactions."
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package revalue

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
)

// Default cadences for the revaluable manual-asset types, in whole days. These
// intentionally differ from freshness.DefaultWindows for the same types: a house
// is re-estimated quarterly, a vehicle twice a year, crypto weekly (it is volatile
// but manually tracked here, no live feed).
const (
	// PropertyDays is the default revaluation cadence for real estate (quarterly).
	PropertyDays = 90
	// VehicleDays is the default revaluation cadence for vehicles (semi-annual).
	VehicleDays = 180
	// CryptoDays is the default revaluation cadence for crypto holdings (weekly).
	CryptoDays = 7
)

// revaluableTypes are the account types treated as periodically estimated assets
// rather than reconciled cash balances.
var revaluableTypes = map[domain.AccountType]int{
	domain.TypeProperty: PropertyDays,
	domain.TypeVehicle:  VehicleDays,
	domain.TypeCrypto:   CryptoDays,
}

// IsRevaluable reports whether an account type is a manual-asset type driven by a
// revaluation cadence (property, vehicle, crypto).
func IsRevaluable(t domain.AccountType) bool {
	_, ok := revaluableTypes[t]
	return ok
}

// DefaultCadences returns the revaluation cadences as a freshness.Windows, so the
// app can layer them over freshness.DefaultWindows (revalue cadences win for the
// revaluable types). Callers then Merge household overrides on top as usual.
func DefaultCadences() freshness.Windows {
	w := make(freshness.Windows, len(revaluableTypes))
	for t, d := range revaluableTypes {
		w[t] = d
	}
	return w
}

// CadenceDays returns the effective revaluation/staleness cadence for an account,
// in whole days, and whether it is tracked. The per-account override
// (Account.RevalueDays, > 0) always wins; otherwise the account's type cadence
// from windows is used. A result of (n, true) with n > 0 means the account has a
// finite cadence; (_, false) or n <= 0 means it is never considered due.
func CadenceDays(a domain.Account, windows freshness.Windows) (int, bool) {
	return windows.EffectiveWindowDays(a)
}

// IsDue reports whether an account is due for revaluation (or, for non-revaluable
// types, stale) as of now, honouring the per-account RevalueDays override. It
// delegates to freshness.IsStale so there is a single staleness rule across the
// app. Archived accounts and untracked/exempt types are never due; a tracked
// account whose balance has never been confirmed is due immediately.
func IsDue(a domain.Account, windows freshness.Windows, now time.Time) bool {
	return freshness.IsStale(a, windows, now)
}

// NextDue returns the date an account next becomes due for revaluation, and whether
// a finite cadence applies. When no cadence applies the second result is false.
func NextDue(a domain.Account, windows freshness.Windows) (time.Time, bool) {
	days, ok := CadenceDays(a, windows)
	if !ok || days <= 0 {
		return time.Time{}, false
	}
	if a.BalanceAsOf.IsZero() {
		return time.Time{}, true
	}
	return a.BalanceAsOf.AddDate(0, 0, days), true
}
