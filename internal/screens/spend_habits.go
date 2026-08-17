// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendpattern"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// habitWindowDays is how far back the habit questions look.
//
// A hundred and twenty days — four months, so the consistency check has enough
// periods to tell a habit from one loud month, and recent enough that a routine
// somebody has already changed drops out of it.
const habitWindowDays = 120

// habitPaydayWindowDays is how long "just after payday" lasts.
//
// Three days beyond the day itself. The question is whether money leaves as soon
// as it lands; stretch it much further and the phase swallows half the month,
// at which point it is being compared against its own leftovers.
const habitPaydayWindowDays = 3

// spendHabitLines describes the spending habits worth telling somebody about
// (WF-SM2). Empty when nothing holds — which is the common and correct answer.
func spendHabitLines(txns []domain.Transaction, rates currency.Rates, base string, now time.Time) []string {
	days, paydays := habitSeries(txns, rates, now)
	if len(days) == 0 {
		return nil
	}
	var out []string

	wIn, wOut := spendpattern.SplitBy(days, spendpattern.IsWeekend)
	if f := spendpattern.Compare(wIn, wOut, spendpattern.MonthKey); f.Holds() {
		out = append(out, uistate.T("habits.weekend",
			int64(f.LiftPct+0.5), fmtMoney(money.New(f.ExtraPerDayMinor(), base)), f.PeriodsHeld, f.Periods))
	}

	if len(paydays) > 0 {
		phase := spendpattern.WithinDaysAfter(paydays, habitPaydayWindowDays)
		pIn, pOut := spendpattern.SplitBy(days, phase)
		if f := spendpattern.Compare(pIn, pOut, spendpattern.MonthKey); f.Holds() {
			out = append(out, uistate.T("habits.payday",
				habitPaydayWindowDays+1, int64(f.LiftPct+0.5),
				fmtMoney(money.New(f.ExtraPerDayMinor(), base)), f.PeriodsHeld, f.Periods))
		}
	}
	return out
}

// habitSeries builds one entry per DAY in the window — including the days
// nothing was spent — plus the dates income actually landed.
//
// The zero days are the point. Skipping them would measure how big a spending
// day is rather than how often money goes out, and those are different
// questions: a household that spends once a week in large amounts would read as
// having the same habit as one that spends every day.
//
// Income dates come from the ledger rather than a pay-cycle setting, because the
// question is when money ACTUALLY arrived; a stated cadence describes an
// intention, and a late paycheque moves the phase it is meant to mark.
func habitSeries(txns []domain.Transaction, rates currency.Rates, now time.Time) ([]spendpattern.Day, []time.Time) {
	start := dateutil.DayStart(now.AddDate(0, 0, -habitWindowDays))
	end := dateutil.DayStart(now)

	spend := map[string]int64{}
	var paydays []time.Time
	seenPay := map[string]bool{}
	for _, t := range txns {
		d := dateutil.DayStart(t.Date)
		if d.Before(start) || d.After(end) {
			continue
		}
		key := d.Format("2006-01-02")
		switch {
		case t.IsExpense() && t.CountsInReports():
			conv, err := rates.Convert(t.Amount, rates.Base)
			if err != nil {
				continue
			}
			spend[key] += conv.Abs().Amount
		case t.IsIncome() && t.CountsInReports():
			if !seenPay[key] {
				seenPay[key] = true
				paydays = append(paydays, d)
			}
		}
	}

	var days []spendpattern.Day
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		days = append(days, spendpattern.Day{Date: d, Minor: spend[d.Format("2006-01-02")]})
	}
	return days, paydays
}
