// SPDX-License-Identifier: MIT

// Package appstate — surplus-sweep scheduled job (C184).
//
// This file implements RunDueSweeps, the monthly surplus-sweep automation that
// moves leftover cash ("surplus") from a checking/source account to a savings
// destination account. It mirrors the pattern of RunDueFundAccruals in
// appstate.go: once-per-period guard, balance check, transfer pair, persist.
//
// No syscall/js dependency; the file may be unit-tested on native Go.
package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/savings"
)

// SweepConfig holds the caller-supplied sweep preferences. It mirrors the
// relevant fields from prefs.Prefs so that RunDueSweeps can be called from
// the wasm layer (which reads prefs) without introducing a syscall/js import
// into appstate itself, keeping the package unit-testable on native Go.
type SweepConfig struct {
	// Enabled must be true for RunDueSweeps to do anything.
	Enabled bool
	// FromAccountID is the source account ID (typically a checking account).
	FromAccountID string
	// ToAccountID is the destination savings account ID.
	ToAccountID string
	// BufferMinor is the minimum balance (in the source account's minor units)
	// to keep after a sweep. Only the amount above this floor is transferred.
	BufferMinor int64
	// LastPeriod is the savings.PeriodKey("monthly") recorded after the most
	// recent successful sweep. Empty means no sweep has been run yet.
	LastPeriod string
}

// SweepConfigFromPrefs extracts SweepConfig from a Prefs value.
func SweepConfigFromPrefs(p prefs.Prefs) SweepConfig {
	return SweepConfig{
		Enabled:       p.SweepEnabled,
		FromAccountID: p.SweepFromAccountID,
		ToAccountID:   p.SweepToAccountID,
		BufferMinor:   p.SweepBufferMinor,
		LastPeriod:    p.SweepLastPeriod,
	}
}

// sweepAmount returns the amount to transfer given the source account's
// current liquid balance and the configured buffer floor. It is a pure
// helper exposed for unit tests.
//
//   - liquid − buffer gives the transferable surplus.
//   - If the result is ≤ 0 (balance at or below the floor), 0 is returned.
func sweepAmount(liquid, buffer int64) int64 {
	if liquid <= buffer {
		return 0
	}
	return liquid - buffer
}

// sweepDue reports whether a sweep is due for the given period key. A sweep is
// due when lastPeriod does not match nowKey (i.e. this month has not been swept
// yet). It is a pure helper exposed for unit tests.
func sweepDue(lastPeriod, nowKey string) bool {
	return lastPeriod != nowKey
}

// RunDueSweeps executes the monthly surplus-sweep if all of the following hold:
//
//  1. cfg.Enabled is true and both account IDs are set and differ.
//  2. The current month has not already been swept (once-per-month guard via
//     savings.PeriodKey).
//  3. The source account's liquid balance exceeds cfg.BufferMinor (the floor).
//
// On success it records a transfer pair via CreateTransferPair and updates
// p.SweepLastPeriod via the supplied persist callback — the caller (wasm
// scheduledworkflows.go) writes the updated prefs back to localStorage.
//
// Returns (1, updatedPrefs, nil) when a sweep was executed, (0, p, nil) when
// skipped, or (0, p, err) on error.
func (a *App) RunDueSweeps(now time.Time, cfg SweepConfig, p prefs.Prefs) (int, prefs.Prefs, error) {
	// Guard: feature must be enabled with distinct, non-empty account IDs.
	if !cfg.Enabled {
		return 0, p, nil
	}
	if cfg.FromAccountID == "" || cfg.ToAccountID == "" {
		return 0, p, nil
	}
	if cfg.FromAccountID == cfg.ToAccountID {
		return 0, p, nil
	}

	// Once-per-month guard.
	nowKey := savings.PeriodKey(now, "monthly")
	if !sweepDue(cfg.LastPeriod, nowKey) {
		return 0, p, nil
	}

	// Resolve source account.
	var fromAcc, _ = findAccount(a, cfg.FromAccountID)
	if fromAcc.ID == "" {
		return 0, p, fmt.Errorf("appstate: sweep: source account %q not found", cfg.FromAccountID)
	}

	// Compute liquid balance of the source account.
	bal, err := ledger.Balance(fromAcc, a.Transactions())
	if err != nil {
		return 0, p, fmt.Errorf("appstate: sweep: balance for account %q: %w", cfg.FromAccountID, err)
	}

	// Determine the surplus above the buffer floor.
	amt := sweepAmount(bal.Amount, cfg.BufferMinor)
	if amt <= 0 {
		// Balance at or below the buffer — nothing to sweep.
		a.log.Info("surplus sweep: balance at or below buffer floor, skipping",
			"account", cfg.FromAccountID,
			"balance", bal.Amount,
			"buffer", cfg.BufferMinor,
		)
		return 0, p, nil
	}

	// Execute the transfer.
	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: cfg.FromAccountID,
		ToAccountID:   cfg.ToAccountID,
		AmountMinor:   amt,
		Desc:          "Surplus sweep",
	}); err != nil {
		return 0, p, fmt.Errorf("appstate: sweep: create transfer: %w", err)
	}

	// Advance the once-per-month guard in the prefs copy.
	p.SweepLastPeriod = nowKey

	a.log.Info("surplus sweep executed",
		"from", cfg.FromAccountID,
		"to", cfg.ToAccountID,
		"amount", amt,
		"period", nowKey,
	)
	return 1, p, nil
}

// findAccount looks up an account by ID from the live store.
// Returns the account and true if found, or a zero value and false otherwise.
func findAccount(a *App, id string) (domain.Account, bool) {
	for _, ac := range a.Accounts() {
		if ac.ID == id {
			return ac, true
		}
	}
	return domain.Account{}, false
}
