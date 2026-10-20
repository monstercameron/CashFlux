// SPDX-License-Identifier: MIT

package assistantview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendpattern"
)

// HabitWindowDays is the number of prior calendar days examined for habits.
const HabitWindowDays = 120

// HabitPaydayWindowDays is the number of days after payday in its spending phase.
// Payday itself is also included.
const HabitPaydayWindowDays = 3

// HabitFacts contains the repeatable weekend and payday spending comparisons.
// Callers format only findings whose Holds method returns true.
type HabitFacts struct {
	Weekend spendpattern.Finding
	Payday  spendpattern.Finding
}

// Habits compares daily spending across the same four calendar months for both
// phases. Daily zeroes remain observations, so occasional spend does not become
// a claim that spending happens every day.
func Habits(txns []domain.Transaction, rates currency.Rates, now time.Time) HabitFacts {
	days, paydays := HabitSeries(txns, rates, now)
	inside, outside := spendpattern.SplitBy(days, spendpattern.IsWeekend)
	out := HabitFacts{Weekend: spendpattern.Compare(inside, outside, spendpattern.MonthKey)}
	if len(paydays) > 0 {
		inside, outside = spendpattern.SplitBy(days, spendpattern.WithinDaysAfter(paydays, HabitPaydayWindowDays))
		out.Payday = spendpattern.Compare(inside, outside, spendpattern.MonthKey)
	}
	return out
}

// HabitSeries builds one observation per calendar day, including the day exactly
// HabitWindowDays before now and today, plus each reported income date once.
// Transfers, excluded reports and invalid expense conversions are omitted.
// An expense that would overflow its day's total is omitted without wrapping it.
func HabitSeries(txns []domain.Transaction, rates currency.Rates, now time.Time) ([]spendpattern.Day, []time.Time) {
	start := dateutil.DayStart(now.AddDate(0, 0, -HabitWindowDays))
	end := dateutil.DayStart(now)
	spend := make(map[string]int64)
	var paydays []time.Time
	seenPay := make(map[string]bool)
	for _, txn := range txns {
		date := dateutil.DayStart(txn.Date)
		if date.Before(start) || date.After(end) {
			continue
		}
		key := date.Format("2006-01-02")
		switch {
		case txn.IsExpense() && txn.CountsInReports():
			converted, err := rates.Convert(txn.Amount, rates.Base)
			if err != nil {
				continue
			}
			magnitude, err := converted.AbsChecked()
			if err != nil {
				continue
			}
			if total, err := money.Add(spend[key], magnitude.Amount); err == nil {
				spend[key] = total
			}
		case txn.IsIncome() && txn.CountsInReports():
			if !seenPay[key] {
				seenPay[key] = true
				paydays = append(paydays, date)
			}
		}
	}
	days := make([]spendpattern.Day, 0, HabitWindowDays+1)
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		days = append(days, spendpattern.Day{Date: date, Minor: spend[date.Format("2006-01-02")]})
	}
	return days, paydays
}
