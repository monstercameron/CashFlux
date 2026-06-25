// SPDX-License-Identifier: MIT

// Package budgeting computes spending against budgets: how much has been spent
// in a budget's category over a period (scope-aware), how much remains, and
// whether the budget is on track, near its limit, or over.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// DefaultNearThreshold is the fraction of the limit at which a budget is "near"
// (but not yet over).
const DefaultNearThreshold = 0.8

// State summarizes how a budget is tracking.
type State string

const (
	StateOK   State = "ok"
	StateNear State = "near"
	StateOver State = "over"
)

// Status is the evaluated state of a budget for a period.
type Status struct {
	Budget    domain.Budget
	Spent     money.Money
	Remaining money.Money
	Percent   int // spent as a percent of the limit (may exceed 100)
	State     State
}

// normalizedLimit returns the budget's limit, defaulting an empty currency to
// the rate table's base currency.
func normalizedLimit(budget domain.Budget, rates currency.Rates) money.Money {
	limit := budget.Limit
	if limit.Currency == "" {
		return money.New(limit.Amount, rates.Base)
	}
	return limit
}

// matchesScope reports whether a transaction is in scope for the budget
// independent of its category: it must be an expense within [start, end), and for
// an individual budget it must belong to the owning member. The per-category test
// is applied separately (on the transaction's category, or per split line for a
// split transaction) by spentCovered.
func matchesScope(budget domain.Budget, t domain.Transaction, start, end time.Time) bool {
	if !t.IsExpense() {
		return false
	}
	if !dateutil.InRange(t.Date, start, end) {
		return false
	}
	if budget.Scope == domain.ScopeIndividual && t.MemberID != budget.OwnerID {
		return false
	}
	return true
}

// spentCovered sums spend against the budget for transactions whose category
// passes covers, in the budget's limit currency. For a split transaction (C58)
// each split line is attributed to its own category — only the lines whose
// category passes covers count, and never the whole-transaction category — so a
// grocery receipt split into produce/household lands in the right budgets without
// double-counting.
func spentCovered(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, covers func(string) bool) (money.Money, error) {
	limit := normalizedLimit(budget, rates)
	total := money.Zero(limit.Currency)
	add := func(amt money.Money) error {
		conv, err := rates.Convert(amt.Abs(), limit.Currency)
		if err != nil {
			return err
		}
		total, err = total.Add(conv)
		return err
	}
	for _, t := range all {
		if !matchesScope(budget, t, start, end) {
			continue
		}
		if t.HasSplits() {
			for _, s := range t.Splits {
				if !covers(s.CategoryID) {
					continue
				}
				if err := add(s.Amount); err != nil {
					return money.Money{}, err
				}
			}
			continue
		}
		if !covers(t.CategoryID) {
			continue
		}
		if err := add(t.Amount); err != nil {
			return money.Money{}, err
		}
	}
	return total, nil
}

// Spent returns the total spent against a budget within [start, end), in the
// budget's limit currency (the budget's own category only).
func Spent(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates) (money.Money, error) {
	return spentCovered(budget, all, start, end, rates, func(id string) bool { return id == budget.CategoryID })
}

// evaluateWith builds the Status using the given category-cover predicate.
func evaluateWith(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64, covers func(string) bool) (Status, error) {
	limit := normalizedLimit(budget, rates)
	spent, err := spentCovered(budget, all, start, end, rates, covers)
	if err != nil {
		return Status{}, err
	}
	remaining, err := limit.Sub(spent)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Budget:    budget,
		Spent:     spent,
		Remaining: remaining,
		Percent:   percent(spent, limit),
		State:     classify(spent, limit, nearThreshold),
	}, nil
}

// Evaluate returns the full Status for a budget over [start, end), counting only
// the budget's own category. nearThreshold is the fraction of the limit
// considered "near"; pass DefaultNearThreshold for the standard 80%.
func Evaluate(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64) (Status, error) {
	return evaluateWith(budget, all, start, end, rates, nearThreshold, func(id string) bool { return id == budget.CategoryID })
}

// EvaluateRollup is like Evaluate but the budget also counts spend in any
// category in covers — typically the budget's category plus its descendants
// (from categorytree.Descendants) — so a parent-category budget includes its
// sub-categories' spend (D5). An empty covers falls back to the budget's own
// category.
func EvaluateRollup(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64, covers map[string]bool) (Status, error) {
	return evaluateWith(budget, all, start, end, rates, nearThreshold, func(id string) bool {
		return id == budget.CategoryID || covers[id]
	})
}

// PeriodRange returns the half-open [start, end) range for the budget period of
// kind p that contains ref. weekStart sets the first day of the week for weekly
// periods. An unknown period falls back to monthly.
func PeriodRange(p domain.Period, ref time.Time, weekStart time.Weekday) (start, end time.Time) {
	switch p {
	case domain.PeriodWeekly:
		start = dateutil.WeekStart(ref, weekStart)
		return start, start.AddDate(0, 0, 7)
	case domain.PeriodQuarterly:
		y, m, _ := ref.Date()
		qm := ((int(m)-1)/3)*3 + 1
		start = time.Date(y, time.Month(qm), 1, 0, 0, 0, 0, ref.Location())
		return start, dateutil.AddMonths(start, 3)
	default:
		return dateutil.MonthRange(ref)
	}
}

// EvaluateAll evaluates a set of budgets over the same period.
func EvaluateAll(budgets []domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64) ([]Status, error) {
	out := make([]Status, 0, len(budgets))
	for _, b := range budgets {
		s, err := Evaluate(b, all, start, end, rates, nearThreshold)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func percent(spent, limit money.Money) int {
	if limit.Amount <= 0 {
		if spent.Amount > 0 {
			return 100
		}
		return 0
	}
	return int(spent.Amount * 100 / limit.Amount)
}

func classify(spent, limit money.Money, nearThreshold float64) State {
	if limit.Amount <= 0 {
		if spent.Amount > 0 {
			return StateOver
		}
		return StateOK
	}
	if spent.Amount >= limit.Amount {
		return StateOver
	}
	if float64(spent.Amount) >= nearThreshold*float64(limit.Amount) {
		return StateNear
	}
	return StateOK
}

// IsDuplicateBudget reports whether adding a budget for the given (categoryID,
// period, ownerID) triple would create a second live budget with the same scope.
// The "one budget per category per period per owner" rule prevents ambiguous
// spend attribution: two budgets competing for the same category + period + owner
// would both accrue the same transactions, making their totals misleading.
//
// It ignores the existing budget whose ID matches excludeID (pass "" to check
// against all). Pass the ID of the budget being edited to allow a save of its
// own unchanged triple.
func IsDuplicateBudget(existing []domain.Budget, categoryID, period, ownerID, excludeID string) bool {
	for _, b := range existing {
		if b.ID == excludeID {
			continue
		}
		if b.CategoryID == categoryID && string(b.Period) == period && b.OwnerID == ownerID {
			return true
		}
	}
	return false
}
