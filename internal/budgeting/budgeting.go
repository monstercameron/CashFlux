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

// matches reports whether a transaction counts toward the budget: it must be an
// expense in the budget's category within [start, end), and for an individual
// budget it must belong to the owning member.
func matches(budget domain.Budget, t domain.Transaction, start, end time.Time) bool {
	if !t.IsExpense() || t.CategoryID != budget.CategoryID {
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

// Spent returns the total spent against a budget within [start, end), in the
// budget's limit currency.
func Spent(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates) (money.Money, error) {
	limit := normalizedLimit(budget, rates)
	total := money.Zero(limit.Currency)
	for _, t := range all {
		if !matches(budget, t, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount.Abs(), limit.Currency)
		if err != nil {
			return money.Money{}, err
		}
		if total, err = total.Add(conv); err != nil {
			return money.Money{}, err
		}
	}
	return total, nil
}

// Evaluate returns the full Status for a budget over [start, end). nearThreshold
// is the fraction of the limit considered "near"; pass DefaultNearThreshold for
// the standard 80%.
func Evaluate(budget domain.Budget, all []domain.Transaction, start, end time.Time, rates currency.Rates, nearThreshold float64) (Status, error) {
	limit := normalizedLimit(budget, rates)
	spent, err := Spent(budget, all, start, end, rates)
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
