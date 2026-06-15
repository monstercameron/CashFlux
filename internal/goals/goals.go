// Package goals computes savings-goal progress: how much remains, percent
// complete, whether the goal is met, and a read-only projected completion date
// given an assumed monthly contribution.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package goals

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Status is the evaluated progress of a goal.
type Status struct {
	Goal          domain.Goal
	Percent       int         // 0..100 (clamped)
	Remaining     money.Money // amount still needed (never negative)
	Complete      bool
	Projected     time.Time // projected completion date (valid only if HasProjection)
	HasProjection bool
}

// Remaining returns the amount still needed to reach the target (never negative).
func Remaining(goal domain.Goal) (money.Money, error) {
	rem, err := goal.TargetAmount.Sub(goal.CurrentAmount)
	if err != nil {
		return money.Money{}, err
	}
	if rem.IsNegative() {
		return money.Zero(goal.TargetAmount.Currency), nil
	}
	return rem, nil
}

// IsComplete reports whether the current amount has reached the target.
func IsComplete(goal domain.Goal) (bool, error) {
	cmp, err := goal.CurrentAmount.Cmp(goal.TargetAmount)
	if err != nil {
		return false, err
	}
	return cmp >= 0, nil
}

// Percent returns progress toward the target as 0..100 (clamped).
func Percent(goal domain.Goal) int {
	target := goal.TargetAmount.Amount
	current := goal.CurrentAmount.Amount
	if target <= 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	p := int(current * 100 / target)
	switch {
	case p < 0:
		return 0
	case p > 100:
		return 100
	default:
		return p
	}
}

// Project estimates the completion date assuming a fixed monthly contribution,
// counting whole months from `from`. It returns ok=false when no projection is
// possible (non-positive contribution). A goal that is already complete projects
// to `from` with ok=true.
func Project(goal domain.Goal, monthly money.Money, from time.Time) (date time.Time, ok bool, err error) {
	rem, err := Remaining(goal)
	if err != nil {
		return time.Time{}, false, err
	}
	if rem.IsZero() {
		return from, true, nil
	}
	if monthly.Currency != rem.Currency {
		return time.Time{}, false, fmt.Errorf("goals: monthly currency %q != goal currency %q", monthly.Currency, rem.Currency)
	}
	if monthly.Amount <= 0 {
		return time.Time{}, false, nil
	}
	months := int((rem.Amount + monthly.Amount - 1) / monthly.Amount) // ceil division
	return dateutil.AddMonths(from, months), true, nil
}

// Evaluate returns the full Status for a goal given an assumed monthly
// contribution and a reference date.
func Evaluate(goal domain.Goal, monthly money.Money, from time.Time) (Status, error) {
	rem, err := Remaining(goal)
	if err != nil {
		return Status{}, err
	}
	complete, err := IsComplete(goal)
	if err != nil {
		return Status{}, err
	}
	projected, has, err := Project(goal, monthly, from)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Goal:          goal,
		Percent:       Percent(goal),
		Remaining:     rem,
		Complete:      complete,
		Projected:     projected,
		HasProjection: has,
	}, nil
}
