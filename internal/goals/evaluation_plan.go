// SPDX-License-Identifier: MIT

package goals

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Assignment returns the explicit monthly plan, converted to the target currency,
// or the evaluated required pace when no plan is set. Unknown coverage/FX returns
// an error so projections cannot silently use a differently denominated amount.
func (e Evaluation) Assignment(g domain.Goal, rates currency.Rates) (money.Money, bool, error) {
	if e.Err != nil {
		return money.Money{}, false, e.Err
	}
	if !g.IsFinancial() {
		return money.Money{}, false, nil
	}
	if g.MonthlyContribution.Amount > 0 {
		m, err := rates.Convert(g.MonthlyContribution, g.TargetAmount.Currency)
		return m, err == nil, err
	}
	return e.MonthlyNeeded, e.HasMonthlyNeeded, nil
}

// Project computes a linear finish from evaluated coverage. Monthly must use
// the target currency; unknown coverage cannot produce a date.
func (e Evaluation) Project(g domain.Goal, monthly money.Money, from time.Time) (time.Time, bool, error) {
	if e.Err != nil {
		return time.Time{}, false, e.Err
	}
	if !g.IsFinancial() {
		return time.Time{}, false, nil
	}
	if monthly.Currency != g.TargetAmount.Currency {
		return time.Time{}, false, fmt.Errorf("goals: monthly currency %q != goal currency %q", monthly.Currency, g.TargetAmount.Currency)
	}
	rem := e.Coverage.Remaining.Amount
	if rem <= 0 {
		return from, true, nil
	}
	if monthly.Amount <= 0 {
		return time.Time{}, false, nil
	}
	if g.IsPaused(from) {
		from = g.PausedUntil
	}
	payments := rem / monthly.Amount
	if rem%monthly.Amount != 0 {
		payments++
	}
	// Keep the date meaningful and prevent an integer narrowing on WASM.
	if payments > maxWorkableMonths {
		return time.Time{}, false, nil
	}
	return dateutil.AddMonths(from, int(payments)-1), true, nil
}

// SliderRange sizes a contribution planner from known coverage without creating
// a temporary Goal with altered saved amounts or reservation currencies.
func (e Evaluation) SliderRange(g domain.Goal, rates currency.Rates) (lo, hi, step money.Money, ok bool) {
	if e.Err != nil || !g.IsFinancial() || e.Coverage.Remaining.Amount <= 0 {
		return
	}
	rem := e.Coverage.Remaining.Amount
	anchor := e.MonthlyNeeded.Amount
	if !e.HasMonthlyNeeded && g.MonthlyContribution.Amount > 0 {
		m, err := rates.Convert(g.MonthlyContribution, g.TargetAmount.Currency)
		if err != nil {
			return
		}
		anchor = m.Amount
	}
	if anchor <= 0 {
		anchor = rem / 12
	}
	if anchor <= 0 {
		anchor = rem
	}
	lower := max(int64(1), anchor/4)
	upper := rem
	if anchor <= rem/4 {
		upper = anchor * 4
	}
	if upper <= lower {
		// A one-unit gap has one honest candidate rather than an overflowing
		// doubled bound. Native range inputs support equal min and max.
		lower, upper = rem, rem
	}
	st := max(int64(1), roundStep((upper-lower)/20))
	return money.New(lower, g.TargetAmount.Currency), money.New(upper, g.TargetAmount.Currency), money.New(st, g.TargetAmount.Currency), true
}
