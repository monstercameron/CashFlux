// SPDX-License-Identifier: MIT

// Package safespend computes "safe to spend" — the liquid cash a household can
// spend right now without shortchanging its committed obligations (bills due,
// goal contributions, and budgeted spending for the period). It is the single
// source of truth for the figure so every surface (dashboard tile, planning,
// budgets, AI tools) agrees (R15 / consolidates C141–C146).
//
// Pure Go, integer minor units only (no float money), stdlib only — so it unit-
// tests on native Go and the wasm/UI layer just feeds it already-derived inputs.
package safespend

// Breakdown is the explainable result: the inputs that net to SafeToSpend, so a
// surface can show "X liquid − bills − goals − budgets = safe to spend" rather
// than a black-box number (determinism & explainability rule). All money fields
// are minor units of Currency.
type Breakdown struct {
	LiquidCash        int64  // spendable cash (cash/checking/savings), base currency
	BillsDue          int64  // bills/recurring due within the horizon (>= 0)
	GoalContributions int64  // goal set-asides committed this period (>= 0)
	CommittedBudgets  int64  // remaining budgeted spend reserved this period (>= 0)
	SafeToSpend       int64  // LiquidCash − BillsDue − GoalContributions − CommittedBudgets (may be negative)
	IsNegative        bool   // true when SafeToSpend < 0 (overcommitted)
	Currency          string // ISO code the minor units are in
}

// Compute nets liquid cash against the three commitment buckets. Negative inputs
// for the commitment buckets are clamped to 0 (a commitment can't be negative);
// LiquidCash passes through as-is (an overdrawn liquid balance is legitimately
// negative). SafeToSpend can go negative — the UI surfaces that as "−$X over".
func Compute(liquidCash, billsDue, goalContributions, committedBudgets int64, currency string) Breakdown {
	billsDue = clampNonNeg(billsDue)
	goalContributions = clampNonNeg(goalContributions)
	committedBudgets = clampNonNeg(committedBudgets)
	safe := liquidCash - billsDue - goalContributions - committedBudgets
	return Breakdown{
		LiquidCash:        liquidCash,
		BillsDue:          billsDue,
		GoalContributions: goalContributions,
		CommittedBudgets:  committedBudgets,
		SafeToSpend:       safe,
		IsNegative:        safe < 0,
		Currency:          currency,
	}
}

// ComputeCategory returns the evenly-paced amount of a category's remaining
// budget that should still cover the rest of the period: remaining × (daysLeft ÷
// daysInPeriod), floored to whole minor units. It answers "~$X is paced for the
// rest of the period" so a user can tell if they're ahead of or behind an even
// burn. Guards: a non-positive daysInPeriod (or daysLeft) returns 0; daysLeft is
// clamped to [0, daysInPeriod]; a negative remaining (already over) returns 0.
func ComputeCategory(remaining int64, daysLeft, daysInPeriod int) int64 {
	if daysInPeriod <= 0 || daysLeft <= 0 || remaining <= 0 {
		return 0
	}
	if daysLeft > daysInPeriod {
		daysLeft = daysInPeriod
	}
	// int64 math throughout to avoid float money; floor via integer division.
	return remaining * int64(daysLeft) / int64(daysInPeriod)
}

func clampNonNeg(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
