// SPDX-License-Identifier: MIT

// Package valuation derives summary figures from an account's recorded balance
// snapshots (the manual valuation history of an illiquid asset like a home, car,
// or investment). Pure Go, no syscall/js — unit-tested natively.
package valuation

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// MonthToDateChange returns how much an account's value has moved since the start
// of the current calendar month: current minus the value as of month start. The
// month-start value is carried forward from the most recent snapshot dated at or
// before the first of the month; if the account has no snapshot that old (its
// history began this month), the earliest snapshot is used as the baseline instead,
// so a freshly tracked asset still reports its change so far. ok is false only when
// there are no snapshots to compare against. Snapshots need not be pre-sorted.
func MonthToDateChange(snaps []domain.BalanceSnapshot, current money.Money, now time.Time) (change money.Money, ok bool) {
	if len(snaps) == 0 {
		return money.Money{}, false
	}
	monthStart := dateutil.MonthStart(now)

	var (
		haveEarliest bool
		earliestAt   time.Time
		earliestVal  int64
		haveBaseline bool // a snapshot at or before month start
		baselineAt   time.Time
		baselineVal  int64
	)
	for _, s := range snaps {
		if !haveEarliest || s.AsOf.Before(earliestAt) {
			haveEarliest, earliestAt, earliestVal = true, s.AsOf, s.BalanceMinor
		}
		if !s.AsOf.After(monthStart) { // AsOf <= monthStart
			if !haveBaseline || s.AsOf.After(baselineAt) {
				haveBaseline, baselineAt, baselineVal = true, s.AsOf, s.BalanceMinor
			}
		}
	}

	baseline := baselineVal
	if !haveBaseline {
		baseline = earliestVal
	}
	return money.New(current.Amount-baseline, current.Currency), true
}
