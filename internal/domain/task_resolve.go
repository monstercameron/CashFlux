// SPDX-License-Identifier: MIT

package domain

// TaskResolve is an optional, data-driven rule that closes a task automatically
// when the underlying money situation resolves itself — so a to-do like "chase
// the duplicate charge" completes on its own once the refund posts, instead of
// rotting on the list. It is supplied by whatever CREATES the task (a smart
// flag, the assistant, or the workflow engine); manually-created tasks leave it
// nil and stay manual.
//
// Two independent criteria may be set; the task resolves when EITHER is met:
//
//   - Condition is a sandboxed formula over the workflow engine's variable
//     surface (the same txnContext-style vars a workflow condition sees), e.g.
//     "balance_HSBC_updated > 0". Empty means no condition.
//   - A structured matcher (MatchPayee + MatchAmountMinor) fires when a matching
//     transaction posts — the refund-match case: a credit whose payee contains
//     MatchPayee and whose magnitude is within tolerance of MatchAmountMinor.
type TaskResolve struct {
	// Condition is an optional formula-language condition evaluated over the
	// engine variable surface. When it evaluates truthy, the task resolves.
	Condition string `json:"condition,omitempty"`

	// MatchPayee is a case-insensitive substring the posting transaction's payee
	// must contain for the structured matcher to fire. Empty disables the matcher.
	MatchPayee string `json:"matchPayee,omitempty"`

	// MatchAmountMinor is the magnitude (positive minor units) the posting
	// transaction must match within MatchToleranceMinor. Zero disables the
	// amount check (payee alone then suffices).
	MatchAmountMinor int64 `json:"matchAmountMinor,omitempty"`

	// MatchCurrency scopes the amount match to one currency; empty matches any.
	MatchCurrency string `json:"matchCurrency,omitempty"`

	// MatchToleranceMinor is the +/- window around MatchAmountMinor that still
	// counts as a match (absorbs rounding / partial-refund wobble). Zero means an
	// exact magnitude match is required.
	MatchToleranceMinor int64 `json:"matchToleranceMinor,omitempty"`

	// MatchRefund requires the posting transaction to be a credit (positive
	// amount) — the refund case. When false the matcher fires on either sign.
	MatchRefund bool `json:"matchRefund,omitempty"`
}

// HasMatcher reports whether the structured transaction matcher is configured.
func (r TaskResolve) HasMatcher() bool { return r.MatchPayee != "" || r.MatchAmountMinor > 0 }

// IsEmpty reports whether the resolve rule carries no criteria at all (so the
// task can never self-resolve and is effectively manual).
func (r TaskResolve) IsEmpty() bool { return r.Condition == "" && !r.HasMatcher() }
