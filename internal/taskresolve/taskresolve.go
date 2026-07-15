// SPDX-License-Identifier: MIT

// Package taskresolve decides whether a self-resolving task (domain.TaskResolve)
// should auto-complete given a data event — a transaction posting, an account
// reconcile, a recurring deletion. It is pure and deterministic: the same event
// against the same rule always yields the same answer, so it unit-tests on
// native Go and the wasm app layer only has to feed it the current context on
// each store mutation (never a timer).
//
// Two criteria compose with OR semantics (see domain.TaskResolve): a formula
// Condition over the engine variable surface, and a structured transaction
// matcher (payee + amount, the refund case). A task resolves the moment either
// is satisfied.
package taskresolve

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// Event is the data context a resolve rule is evaluated against on a store
// mutation. Vars and Strs are the workflow engine's variable surface (so a
// Condition can reference the same names a workflow condition sees). Txn, when
// non-nil, is the transaction that just posted, for the structured matcher.
type Event struct {
	Vars map[string]float64
	Strs map[string]string
	Txn  *TxnEvent
}

// TxnEvent describes the transaction behind a posting event for the structured
// matcher. AmountMinor is signed (negative = money out, positive = a credit /
// refund) in the transaction's own currency minor units.
type TxnEvent struct {
	Payee       string
	AmountMinor int64
	Currency    string
}

// Resolves reports whether the resolve rule is satisfied by the event (and thus
// the task should auto-complete). An empty rule never resolves. A malformed
// Condition returns the evaluation error and does NOT resolve — a broken rule
// must never silently close a task.
func Resolves(r domain.TaskResolve, e Event) (bool, error) {
	if r.IsEmpty() {
		return false, nil
	}
	if r.HasMatcher() && matches(r, e.Txn) {
		return true, nil
	}
	if strings.TrimSpace(r.Condition) != "" {
		ok, err := workflow.Eval(r.Condition, workflow.Context{Vars: e.Vars, Strs: e.Strs})
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// matches reports whether the structured matcher fires for the given posting
// transaction. It requires a payee substring match (case-insensitive) when
// MatchPayee is set, an amount-magnitude match within tolerance when
// MatchAmountMinor is set, a currency match when MatchCurrency is set, and a
// positive (credit) sign when MatchRefund is set.
func matches(r domain.TaskResolve, t *TxnEvent) bool {
	if t == nil {
		return false
	}
	if r.MatchCurrency != "" && !strings.EqualFold(r.MatchCurrency, t.Currency) {
		return false
	}
	if r.MatchPayee != "" && !strings.Contains(strings.ToLower(t.Payee), strings.ToLower(r.MatchPayee)) {
		return false
	}
	if r.MatchRefund && t.AmountMinor <= 0 {
		return false
	}
	if r.MatchAmountMinor > 0 {
		mag := t.AmountMinor
		if mag < 0 {
			mag = -mag
		}
		diff := mag - r.MatchAmountMinor
		if diff < 0 {
			diff = -diff
		}
		if diff > r.MatchToleranceMinor {
			return false
		}
	}
	return true
}
