// SPDX-License-Identifier: MIT

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
	OnTrack       bool // projected to be met on/before the target date (valid only if PaceKnown)
	PaceKnown     bool // whether OnTrack is meaningful (goal has a target date and a usable projection)
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

// MonthlyNeeded returns the contribution per remaining month required to reach the
// goal by its TargetDate, counting whole months from `from` (rounding a partial
// month up, minimum one). It returns ok=false when the goal has no target date, is
// already complete, or the target date is not in the future.
func MonthlyNeeded(goal domain.Goal, from time.Time) (money.Money, bool, error) {
	if goal.TargetDate.IsZero() || !goal.TargetDate.After(from) {
		return money.Money{}, false, nil
	}
	rem, err := Remaining(goal)
	if err != nil {
		return money.Money{}, false, err
	}
	if rem.IsZero() {
		return money.Money{}, false, nil
	}
	months := (goal.TargetDate.Year()-from.Year())*12 + int(goal.TargetDate.Month()) - int(from.Month())
	if goal.TargetDate.Day() > from.Day() {
		months++ // a partial final month still needs a contribution
	}
	if months < 1 {
		months = 1
	}
	per := (rem.Amount + int64(months) - 1) / int64(months) // ceil division
	return money.New(per, rem.Currency), true, nil
}

// OnTrack reports whether, at the given monthly contribution, the goal is
// projected to be met on or before its target date. known is false when there's
// nothing to judge — the goal has no target date, or no projection is possible
// (a non-positive contribution on an unmet goal). An already-complete goal is on
// track. It's the pace check that complements MonthlyNeeded ("how much to stay on
// schedule") with "am I on schedule at this rate?".
func OnTrack(goal domain.Goal, monthly money.Money, from time.Time) (onTrack, known bool, err error) {
	if goal.TargetDate.IsZero() {
		return false, false, nil
	}
	complete, err := IsComplete(goal)
	if err != nil {
		return false, false, err
	}
	if complete {
		return true, true, nil
	}
	projected, ok, err := Project(goal, monthly, from)
	if err != nil {
		return false, false, err
	}
	if !ok {
		return false, false, nil
	}
	return !projected.After(goal.TargetDate), true, nil
}

// Overfund returns the surplus amount by which a goal's current balance exceeds
// its target. When the goal is not over-funded (or exactly at target) it returns
// a zero-valued Money in the goal's target currency. Currency mismatches are
// propagated as errors.
func Overfund(goal domain.Goal) (money.Money, error) {
	cmp, err := goal.CurrentAmount.Cmp(goal.TargetAmount)
	if err != nil {
		return money.Money{}, err
	}
	if cmp <= 0 {
		return money.Zero(goal.TargetAmount.Currency), nil
	}
	surplus, err := goal.CurrentAmount.Sub(goal.TargetAmount)
	if err != nil {
		return money.Money{}, err
	}
	return surplus, nil
}

// OverallProgress computes the combined 0..100 savings progress across the
// provided goals. When includeArchived is false, archived goals are excluded
// from both the numerator and denominator — they no longer dilute the headline
// figure after being moved to the "Achieved" section. A zero total target
// returns 0. The result is clamped to 100.
func OverallProgress(goals []domain.Goal, includeArchived bool) (int, error) {
	var saved, target int64
	for _, g := range goals {
		if !includeArchived && g.Archived {
			continue
		}
		saved += g.CurrentAmount.Amount
		target += g.TargetAmount.Amount
	}
	if target <= 0 {
		return 0, nil
	}
	pct := int(saved * 100 / target)
	if pct > 100 {
		return 100, nil
	}
	if pct < 0 {
		return 0, nil
	}
	return pct, nil
}

// MilestoneCrossed reports whether a contribution that moved the goal's progress
// from beforePct to afterPct crossed one of the celebratory milestones (25, 50,
// 75, or 100 percent). It returns the highest milestone crossed, or 0 if none.
// Both inputs are clamped to 0..100 before comparison. This is used to decide
// whether to show a milestone toast after a contribution.
//
// Examples:
//
//	MilestoneCrossed(20, 30) → 25   (crossed 25%)
//	MilestoneCrossed(20, 60) → 50   (crossed both 25% and 50%; highest returned)
//	MilestoneCrossed(50, 60) → 0    (no milestone crossed)
//	MilestoneCrossed(0, 100) → 100  (crossed 25, 50, 75, 100; highest returned)
func MilestoneCrossed(beforePct, afterPct int) int {
	if beforePct < 0 {
		beforePct = 0
	}
	if afterPct > 100 {
		afterPct = 100
	}
	highest := 0
	for _, m := range []int{25, 50, 75, 100} {
		if beforePct < m && afterPct >= m {
			highest = m
		}
	}
	return highest
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
	// Derive pace from the values already computed (no second Project call):
	// a dated goal is on track if complete, or projected on/before its target.
	onTrack, paceKnown := false, false
	if !goal.TargetDate.IsZero() {
		switch {
		case complete:
			onTrack, paceKnown = true, true
		case has:
			onTrack, paceKnown = !projected.After(goal.TargetDate), true
		}
	}
	return Status{
		Goal:          goal,
		Percent:       Percent(goal),
		Remaining:     rem,
		Complete:      complete,
		Projected:     projected,
		HasProjection: has,
		OnTrack:       onTrack,
		PaceKnown:     paceKnown,
	}, nil
}
