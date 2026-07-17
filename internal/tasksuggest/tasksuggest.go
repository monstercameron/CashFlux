// SPDX-License-Identifier: MIT

// Package tasksuggest scans the household's data for unresolved conditions
// worth a to-do — stale account balances, a pile of unreviewed transactions,
// overspent budgets — and proposes tasks for them. Deterministic suggestions,
// never silent creation: the UI shows each proposal with Add/Dismiss, and the
// added task carries a TaskResolve condition where the engine variable surface
// can express "this condition cleared" (so taskresolve auto-completes it).
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package tasksuggest

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/freshness"
)

// Kind names a suggestion's condition family.
type Kind string

const (
	KindStaleAccount    Kind = "stale-account"
	KindUnreviewed      Kind = "unreviewed"
	KindOverspentBudget Kind = "overspent-budget"
)

// UnreviewedThreshold is how many unreviewed transactions it takes before the
// review-backlog suggestion appears — a couple of pending rows is normal life,
// a pile is a chore worth scheduling.
const UnreviewedThreshold = 5

// maxPerKind caps how many same-kind suggestions surface at once so a neglected
// dataset proposes a short list, not a wall.
const maxPerKind = 3

// Suggestion is one proposed task. Key is the stable dismissal identity (same
// condition → same key, so a dismissal holds until the condition changes
// entity). Name/Count parameterize the localized title at the UI layer.
type Suggestion struct {
	Key         string
	Kind        Kind
	Name        string // account / budget name ("" for aggregate kinds)
	Count       int    // e.g. unreviewed transactions
	RelatedType domain.RelatedType
	RelatedID   string
	// Resolve, when non-nil, auto-completes the created task once the condition
	// clears (taskresolve). Nil = manual close (no engine var can express it).
	Resolve *domain.TaskResolve
	// DueDays is the proposed due date, days from now.
	DueDays int
}

// Scan produces the current suggestions in a stable order (stale accounts,
// review backlog, overspent budgets).
func Scan(accounts []domain.Account, txns []domain.Transaction, budgets []domain.Budget,
	windows freshness.Windows, rates currency.Rates, now time.Time, weekStart time.Weekday) []Suggestion {

	var out []Suggestion

	// Stale balances: one suggestion per stale account (capped). No engine var
	// expresses "this account was updated", so these close manually — or vanish
	// from the strip the moment the balance is confirmed.
	for i, a := range freshness.StaleAccounts(accounts, windows, now) {
		if i >= maxPerKind {
			break
		}
		out = append(out, Suggestion{
			Key: "stale:" + a.ID, Kind: KindStaleAccount, Name: a.Name,
			RelatedType: domain.RelatedAccount, RelatedID: a.ID, DueDays: 3,
		})
	}

	// Review backlog: a single aggregate suggestion once the pile clears the
	// threshold. Resolves itself when txns_unreviewed hits zero.
	unreviewed := 0
	for _, t := range txns {
		if !t.Reviewed && !t.IsTransfer() {
			unreviewed++
		}
	}
	if unreviewed >= UnreviewedThreshold {
		out = append(out, Suggestion{
			Key: "unreviewed", Kind: KindUnreviewed, Count: unreviewed,
			Resolve: &domain.TaskResolve{Condition: "txns_unreviewed == 0"},
			DueDays: 7,
		})
	}

	// Overspent budgets: one per budget (capped), resolving via the budget's own
	// engine variable (budget_<slug>_over) the moment it stops being overspent.
	over := 0
	for _, base := range engineenv.BudgetVarBases(budgets) {
		if over >= maxPerKind {
			break
		}
		b := base.Budget
		start, end := budgeting.PeriodRange(b.Period, now, weekStart)
		spent, err := budgeting.Spent(b, txns, start, end, rates)
		if err != nil || spent.Amount <= b.Limit.Amount || b.Limit.Amount <= 0 {
			continue
		}
		out = append(out, Suggestion{
			Key: "overspent:" + b.ID, Kind: KindOverspentBudget, Name: b.Name,
			RelatedType: domain.RelatedBudget, RelatedID: b.ID,
			Resolve: &domain.TaskResolve{Condition: fmt.Sprintf("%sover <= 0", base.Prefix)},
			DueDays: 5,
		})
		over++
	}

	return out
}
