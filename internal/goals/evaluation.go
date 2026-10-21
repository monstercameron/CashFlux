// SPDX-License-Identifier: MIT

package goals

import (
	"fmt"
	"math/bits"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// CoverageEvaluation expresses saved and reserved money in the target currency.
// On error only Saved/SavedPercent are usable, and only when SavedKnown is true.
// It never contains a rewritten Goal that could accidentally be persisted.
type CoverageEvaluation struct {
	Saved, SetAside, Covered, Remaining money.Money
	SavedPercent, CoveragePercent       int
	SavedKnown, Complete                bool
	UnavailableCurrencies               []string
}

// AllocationTotal sums reservations by their stored currency before converting
// each currency total once. This preserves legacy foreign-currency reservations
// and avoids rounding every small reservation separately.
func AllocationTotal(g domain.Goal, rates currency.Rates, to string) (money.Money, error) {
	amounts := map[string]int64{}
	for _, allocation := range g.Allocations {
		if allocation.Amount.Amount == 0 {
			continue
		}
		if allocation.Amount.Amount < 0 {
			return money.Money{}, fmt.Errorf("goals: negative reservation on goal %s", g.ID)
		}
		code := allocation.Amount.Currency
		total, err := money.Add(amounts[code], allocation.Amount.Amount)
		if err != nil {
			return money.Money{}, fmt.Errorf("goals: sum goal %s reservations: %w", g.ID, err)
		}
		amounts[code] = total
	}
	// Stable addition order also makes overflow failures deterministic.
	codes := make([]string, 0, len(amounts))
	for code := range amounts {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	total := money.Zero(to)
	for _, code := range codes {
		converted, err := rates.Convert(money.New(amounts[code], code), to)
		if err == nil {
			converted, err = total.Add(converted)
		}
		if err != nil {
			return money.Money{}, fmt.Errorf("goals: convert goal %s reservations in %s: %w", g.ID, code, err)
		}
		total = converted
	}
	return total, nil
}

// EvaluateCoverage checks saved money, reservations and all arithmetic. Missing
// FX and overflow return an explicit error, never a relabeled amount or a false
// completion. An unspecified currency is accepted only for legacy zero savings.
func EvaluateCoverage(g domain.Goal, rates currency.Rates) (CoverageEvaluation, error) {
	var c CoverageEvaluation
	saved := g.CurrentAmount
	if saved.Currency == "" && saved.Amount == 0 {
		saved = money.Zero(g.TargetAmount.Currency)
	}
	var err error
	c.Saved, err = rates.Convert(saved, g.TargetAmount.Currency)
	if err != nil {
		c.UnavailableCurrencies = []string{saved.Currency}
		return c, fmt.Errorf("goals: goal %s saved amount: %w", g.ID, err)
	}
	c.SavedKnown = true
	c.SavedPercent = boundedPercent(c.Saved.Amount, g.TargetAmount.Amount)
	c.SetAside, err = AllocationTotal(g, rates, g.TargetAmount.Currency)
	if err == nil {
		c.Covered, err = c.Saved.Add(c.SetAside)
	}
	if err == nil {
		c.Remaining, err = g.TargetAmount.Sub(c.Covered)
	}
	if err != nil {
		codes := map[string]bool{g.TargetAmount.Currency: true}
		for _, a := range g.Allocations {
			if a.Amount.Amount != 0 {
				codes[a.Amount.Currency] = true
			}
		}
		for code := range codes {
			c.UnavailableCurrencies = append(c.UnavailableCurrencies, code)
		}
		sort.Strings(c.UnavailableCurrencies)
		return c, fmt.Errorf("goals: goal %s coverage: %w", g.ID, err)
	}
	if c.Remaining.Amount < 0 {
		c.Remaining = money.Zero(g.TargetAmount.Currency)
	}
	c.CoveragePercent = boundedPercent(c.Covered.Amount, g.TargetAmount.Amount)
	c.Complete = g.TargetAmount.Amount > 0 && c.Covered.Amount >= g.TargetAmount.Amount
	return c, nil
}

// Evaluation is the canonical kind-aware read model. Err makes coverage-derived
// values unknown: Complete remains false, State remains Current, and no pace or
// monthly requirement is inferred. Persist the original Goal, never these facts.
type Evaluation struct {
	Progress         Progress
	Coverage         CoverageEvaluation
	Complete         bool
	State            GoalState
	Pace             Pace
	MonthlyNeeded    money.Money
	HasMonthlyNeeded bool
	Err              error
}

// EvaluateGoal evaluates one goal using a precomputed summary of its linked tasks.
// BuildTaskIndex lets page-level consumers evaluate all goals in O(tasks+goals).
func EvaluateGoal(g domain.Goal, tasks TaskSummary, rates currency.Rates, now time.Time) Evaluation {
	e := Evaluation{State: StateCurrent}
	if g.EffectiveKind() == domain.GoalKindChecklist {
		e.Progress = Progress{Kind: domain.GoalKindChecklist, Done: tasks.Done, Total: tasks.Total,
			Percent: ChecklistPercent(tasks.Done, tasks.Total), Complete: tasks.Total > 0 && tasks.Done >= tasks.Total}
	} else if !g.IsFinancial() {
		e.Progress = EvaluateProgress(g, nil, now)
	} else {
		e.Coverage, e.Err = EvaluateCoverage(g, rates)
		e.Progress = Progress{Kind: domain.GoalKindFinancial, Percent: e.Coverage.SavedPercent}
		if e.Err != nil {
			return e
		}
		e.Progress.Complete = e.Coverage.Saved.Amount >= g.TargetAmount.Amount && g.TargetAmount.Amount > 0
		e.Complete = e.Coverage.Complete
		e.Pace = coveragePace(g, e.Coverage, now)
		e.MonthlyNeeded, e.HasMonthlyNeeded = monthlyNeededForRemaining(g, e.Coverage.Remaining, now)
	}
	if !g.IsFinancial() {
		e.Complete = e.Progress.Complete
	}
	if e.Complete {
		e.State = StateCompleted
	} else if !g.TargetDate.IsZero() && !g.TargetDate.After(now) && !g.IsPaused(now) {
		e.State = StateMissed
	}
	return e
}

func coveragePace(g domain.Goal, c CoverageEvaluation, now time.Time) Pace {
	if g.TargetAmount.Amount <= 0 {
		return PaceNone
	}
	if c.Complete {
		return PaceComplete
	}
	if g.IsPaused(now) {
		return PacePaused
	}
	if !g.TargetDate.IsZero() && !g.TargetDate.After(now) {
		return PaceOverdue
	}
	if c.CoveragePercent >= finalStretchPct {
		return PaceFinalStretch
	}
	if g.TargetDate.IsZero() {
		return PaceNone
	}
	if g.TargetDate.Before(now.AddDate(0, 0, dueSoonDays)) {
		return PaceDueSoon
	}
	return PaceOnTrack
}

func monthlyNeededForRemaining(g domain.Goal, rem money.Money, from time.Time) (money.Money, bool) {
	if g.IsPaused(from) {
		from = g.PausedUntil
	}
	if g.TargetDate.IsZero() || !g.TargetDate.After(from) || rem.Amount <= 0 {
		return money.Money{}, false
	}
	months := (g.TargetDate.Year()-from.Year())*12 + int(g.TargetDate.Month()) - int(from.Month())
	if g.TargetDate.Day() > from.Day() {
		months++
	}
	if months < 1 {
		months = 1
	}
	per := rem.Amount / int64(months)
	if rem.Amount%int64(months) != 0 {
		per++
	}
	return money.New(per, rem.Currency), true
}

func boundedPercent(n, d int64) int {
	if n <= 0 || d <= 0 {
		return 0
	}
	if n >= d {
		return 100
	}
	hi, lo := bits.Mul64(uint64(n), 100)
	p, _ := bits.Div64(hi, lo, uint64(d))
	return int(p)
}
