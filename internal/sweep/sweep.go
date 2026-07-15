// SPDX-License-Identifier: MIT

// Package sweep evaluates per-account surplus-sweep rules (AC7) into PROPOSED
// transfers. A rule says "keep checking at $3,000; move the excess to savings
// monthly"; on its cadence this package computes the sweepable excess and returns
// a Proposal the UI shows for preview-and-approve. Nothing here executes a
// transfer — the app never auto-moves money; it renders the proposal on the same
// approval surface as the payday waterfall (GL1) and only the transfer flow, once
// the user confirms, mutates the ledger.
//
// The excess respects earmark integrity (XC7): money virtually reserved against
// the source account for goals is NOT swept. All amounts are int64 minor units in
// the source account's currency. Pure Go, unit-tested on native Go.
package sweep

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Proposal is a single computed sweep the user can approve. It is plain data; the
// UI turns it into a transfer via the existing transfer flow on approval.
type Proposal struct {
	// RuleID is the sweep rule that produced this proposal.
	RuleID string
	// SourceAccountID / DestAccountID are the transfer legs.
	SourceAccountID string
	DestAccountID   string
	// AmountMinor is the excess to move, in the source account's currency minor
	// units. Always positive when a proposal is returned.
	AmountMinor int64
	// Currency is the source account's currency.
	Currency string
	// KeepMinor / BalanceMinor / EarmarkedMinor are the inputs, retained so the UI
	// can show an explainable breakdown ("balance $5,000 − keep $3,000 − earmarked
	// $500 = sweep $1,500").
	KeepMinor      int64
	BalanceMinor   int64
	EarmarkedMinor int64
}

// Inputs is the per-rule context the caller supplies from the current data
// snapshot: the source account's balance and how much of it is earmarked for goals
// (goals.AccountEarmarkedMinor). Currency is the source account's currency.
type Inputs struct {
	BalanceMinor   int64
	EarmarkedMinor int64
	Currency       string
}

// IsDue reports whether a rule's cadence has elapsed as of now. A disabled rule is
// never due. A rule that has never proposed (empty LastProposed) is due
// immediately. LastProposed is an RFC3339 or date-only ("2006-01-02") string.
func IsDue(r domain.SweepRule, now time.Time) bool {
	if !r.Enabled {
		return false
	}
	last, ok := parseDate(r.LastProposed)
	if !ok {
		return true // never proposed
	}
	return !now.Before(last.AddDate(0, 0, r.Cadence.CadenceDays()))
}

// ExcessMinor returns the sweepable amount for a rule given its inputs, respecting
// the keep floor and earmark integrity (XC7). It is never negative. Earmarked money
// is protected first: only balance above BOTH the keep floor and the earmarked
// reservation is sweepable.
func ExcessMinor(r domain.SweepRule, in Inputs) int64 {
	floor := r.KeepMinor
	if in.EarmarkedMinor > 0 {
		floor += in.EarmarkedMinor
	}
	excess := in.BalanceMinor - floor
	if excess < 0 {
		return 0
	}
	return excess
}

// Propose builds a Proposal for a rule if one is warranted right now: the cadence
// must be due, source and destination must differ, and the excess must clear the
// rule's MinSweepMinor threshold (and be positive). The boolean is false when no
// proposal should surface. now drives the cadence check.
func Propose(r domain.SweepRule, in Inputs, now time.Time) (Proposal, bool) {
	if r.SourceAccountID == "" || r.DestAccountID == "" || r.SourceAccountID == r.DestAccountID {
		return Proposal{}, false
	}
	if !IsDue(r, now) {
		return Proposal{}, false
	}
	excess := ExcessMinor(r, in)
	if excess <= 0 {
		return Proposal{}, false
	}
	min := r.MinSweepMinor
	if min < 1 {
		min = 1
	}
	if excess < min {
		return Proposal{}, false
	}
	return Proposal{
		RuleID:          r.ID,
		SourceAccountID: r.SourceAccountID,
		DestAccountID:   r.DestAccountID,
		AmountMinor:     excess,
		Currency:        in.Currency,
		KeepMinor:       r.KeepMinor,
		BalanceMinor:    in.BalanceMinor,
		EarmarkedMinor:  in.EarmarkedMinor,
	}, true
}

// parseDate parses an RFC3339 or date-only timestamp; ok is false on empty/invalid.
func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}
