// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/assistantview"
	"github.com/monstercameron/CashFlux/internal/currency"
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
const habitWindowDays = assistantview.HabitWindowDays

// habitPaydayWindowDays is how long "just after payday" lasts.
//
// Three days beyond the day itself. The question is whether money leaves as soon
// as it lands; stretch it much further and the phase swallows half the month,
// at which point it is being compared against its own leftovers.
const habitPaydayWindowDays = assistantview.HabitPaydayWindowDays

// spendHabitLines describes the spending habits worth telling somebody about
// (WF-SM2). Empty when nothing holds — which is the common and correct answer.
func spendHabitLines(txns []domain.Transaction, rates currency.Rates, base string, now time.Time) []string {
	facts := assistantview.Habits(txns, rates, now)
	var out []string

	if f := facts.Weekend; f.Holds() {
		out = append(out, uistate.T("habits.weekend",
			int64(f.LiftPct+0.5), fmtMoney(money.New(f.ExtraPerDayMinor(), base)), f.PeriodsHeld, f.Periods))
	}

	if f := facts.Payday; f.Holds() {
		out = append(out, uistate.T("habits.payday", habitPaydayWindowDays+1, int64(f.LiftPct+0.5),
			fmtMoney(money.New(f.ExtraPerDayMinor(), base)), f.PeriodsHeld, f.Periods))
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
	return assistantview.HabitSeries(txns, rates, now)
}
