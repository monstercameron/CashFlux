// SPDX-License-Identifier: MIT

// Package appstate — monthly round-up savings automation (C183).
//
// This file implements RunDueRoundUps, the periodic-batch round-up feature that
// totals the spare-change deltas from all expense transactions in the current
// calendar month and moves that sum to a designated savings account in one
// transfer. It mirrors the pattern of RunDueSweeps in sweep.go: once-per-month
// guard, pure helpers, persist prefs, return count.
//
// No syscall/js dependency; the file may be unit-tested on native Go.
package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/savings"
)

// roundUpDue reports whether a round-up batch is due for the given period key.
// A batch is due when lastPeriod does not match nowKey (i.e. this calendar month
// has not been processed yet). It is a pure helper exposed for unit tests.
func roundUpDue(lastPeriod, nowKey string) bool {
	return lastPeriod != nowKey
}

// roundUpTotal sums the round-up delta for every expense transaction on
// fromAccountID whose date falls in [start, end). Only expense transactions are
// included — income and transfers are skipped. amountMinor is passed as its
// absolute value to RoundUpDelta.
//
// gran is the rounding granularity in minor units (e.g. 100 = nearest dollar).
// If gran is ≤ 0 it is treated as 100.
//
// The function is pure and testable on native Go.
func roundUpTotal(txns []domain.Transaction, fromAccountID string, gran int64, start, end time.Time) int64 {
	if gran <= 0 {
		gran = 100
	}
	var total int64
	for _, t := range txns {
		if t.AccountID != fromAccountID {
			continue
		}
		if !t.IsExpense() {
			continue
		}
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		abs := -t.Amount.Amount // IsExpense guarantees Amount.Amount < 0
		if abs < 0 {
			abs = -abs // paranoia: ensure positive
		}
		total += savings.RoundUpDelta(abs, gran)
	}
	return total
}

// RunDueRoundUps executes the monthly round-up batch if all of the following hold:
//
//  1. p.RoundUpEnabled is true and both account IDs are set and differ.
//  2. The current month has not already been processed (once-per-month guard via
//     savings.PeriodKey).
//  3. The sum of all expense round-up deltas for the month is greater than zero.
//
// On success it creates a single transfer via CreateTransferPair and advances
// p.RoundUpLastPeriod — the caller (scheduledworkflows.go) must persist the
// returned prefs so the guard survives a reload.
//
// Returns (1, updatedPrefs, nil) when a batch was executed, (0, p, nil) when
// skipped, or (0, p, err) on error.
func (a *App) RunDueRoundUps(now time.Time, p prefs.Prefs) (int, prefs.Prefs, error) {
	// Guard: feature must be enabled with distinct, non-empty account IDs.
	if !p.RoundUpEnabled {
		return 0, p, nil
	}
	if p.RoundUpFromAccountID == "" || p.RoundUpToAccountID == "" {
		return 0, p, nil
	}
	if p.RoundUpFromAccountID == p.RoundUpToAccountID {
		return 0, p, nil
	}

	// Once-per-month guard.
	nowKey := savings.PeriodKey(now, "monthly")
	if !roundUpDue(p.RoundUpLastPeriod, nowKey) {
		return 0, p, nil
	}

	// Compute the granularity; default to nearest dollar (100 minor units).
	gran := p.RoundUpGranularityMinor
	if gran <= 0 {
		gran = 100
	}

	// Compute the current month's [start, end) range.
	start, end := dateutil.MonthRange(now)

	// Sum round-up deltas across the month's expense transactions.
	total := roundUpTotal(a.Transactions(), p.RoundUpFromAccountID, gran, start, end)
	if total <= 0 {
		// No spare change this month — skip without marking the guard so we
		// don't permanently block the batch if the month has no expenses yet.
		a.log.Info("round-up batch: no spare change this month, skipping",
			"account", p.RoundUpFromAccountID,
			"period", nowKey,
		)
		return 0, p, nil
	}

	// Validate that the from-account actually exists.
	if _, ok := findAccount(a, p.RoundUpFromAccountID); !ok {
		return 0, p, fmt.Errorf("appstate: round-up: source account %q not found", p.RoundUpFromAccountID)
	}

	// Execute the transfer: one consolidated batch transfer.
	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: p.RoundUpFromAccountID,
		ToAccountID:   p.RoundUpToAccountID,
		AmountMinor:   total,
		Desc:          "Round-up savings",
	}); err != nil {
		return 0, p, fmt.Errorf("appstate: round-up: create transfer: %w", err)
	}

	// Advance the once-per-month guard in the prefs copy.
	p.RoundUpLastPeriod = nowKey

	a.log.Info("round-up batch executed",
		"from", p.RoundUpFromAccountID,
		"to", p.RoundUpToAccountID,
		"amount", total,
		"period", nowKey,
	)
	return 1, p, nil
}
