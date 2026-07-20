// SPDX-License-Identifier: MIT

package runway

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Pay-cycle window bounds for the tideline hero. The window runs from now to the
// next income event, but is floored so a paycheck landing in a day or two still
// yields a readable cushion curve, and capped so a distant (or missing) paycheck
// does not stretch the projection indefinitely. With no income scheduled the
// window degrades to a plain 30-day look-ahead.
const (
	minPinchWindowDays      = 14
	maxPinchWindowDays      = 45
	fallbackPinchWindowDays = 30
)

// Pinch is the tightest point of the projected cushion over a pay cycle — the
// lowest available cash between now and the next income event. Amounts are
// integer minor units in the base currency.
type Pinch struct {
	// AmountMinor is the lowest projected cushion over the window.
	AmountMinor int64
	// Day is the day offset from the projection start where the pinch occurs
	// (0 = today).
	Day int
	// Date is the calendar date of the pinch.
	Date time.Time
	// Negative reports whether the cushion dips below zero at the pinch (the
	// red-flag case; anything positive is at most an amber "tightest" note).
	Negative bool
}

// PayCycle is the tideline's pinch/cushion analysis for the current pay cycle:
// the projected window, the running cushion curve, and its pinch. It reuses the
// same recurring-flow projection the runway/forecast engine uses, so the hero and
// the forecast never disagree.
type PayCycle struct {
	// WindowDays is the projected horizon in days: now → next income event,
	// clamped to [minPinchWindowDays, maxPinchWindowDays]; fallbackPinchWindowDays
	// when no income is scheduled.
	WindowDays int
	// NextIncomeDay is the day offset of the next inbound recurring event that
	// anchored the window, or -1 when none is scheduled within reach (degraded
	// window).
	NextIncomeDay int
	// HasIncome reports whether an income event anchored the window.
	HasIncome bool
	// Cushion is the projected end-of-day cushion for each day in [0, WindowDays).
	Cushion []cashflow.DailyBalance
	// Pinch is the tightest cushion point in the window.
	Pinch Pinch
	// StartMinor is the starting liquid balance used for the projection.
	StartMinor int64
}

// Tideline computes the pay-cycle cushion curve and its pinch from the household's
// liquid balance and recurring cash flows. It finds the next income event to size
// the window (floored/capped, or a 30-day fallback when there is no income),
// projects the running cushion over that window via the shared Project/Events math
// (never a forked projection), and reports the lowest cushion point — the pinch —
// with its date and whether it goes negative.
//
// liquidStart is the current liquid balance (cash + checking) in base minor units;
// from is "now"; rates converts the recurring amounts to the base currency.
func Tideline(liquidStart int64, recs []domain.Recurring, from time.Time, rates currency.Rates) (PayCycle, error) {
	// Look one day past the cap so an income event exactly at the cap still counts.
	lookahead := maxPinchWindowDays + 1
	events, err := Events(recs, from, lookahead, rates)
	if err != nil {
		return PayCycle{}, err
	}

	nextIncome := -1
	for _, e := range events {
		if e.Amount > 0 && (nextIncome < 0 || e.Day < nextIncome) {
			nextIncome = e.Day
		}
	}

	hasIncome := nextIncome >= 0
	window := fallbackPinchWindowDays
	if hasIncome {
		window = nextIncome
		if window < minPinchWindowDays {
			window = minPinchWindowDays
		}
		if window > maxPinchWindowDays {
			window = maxPinchWindowDays
		}
	}

	// Buffer 0: the projection's own min tracking gives us the pinch; a negative
	// minimum is the overdraft case.
	proj, err := Project(liquidStart, recs, from, window, 0, rates)
	if err != nil {
		return PayCycle{}, err
	}

	pinch := Pinch{
		AmountMinor: proj.MinBalance,
		Day:         proj.MinDay,
		Date:        startOfDay(from).AddDate(0, 0, proj.MinDay),
		Negative:    proj.MinBalance < 0,
	}

	return PayCycle{
		WindowDays:    window,
		NextIncomeDay: nextIncome,
		HasIncome:     hasIncome,
		Cushion:       proj.Daily,
		Pinch:         pinch,
		StartMinor:    liquidStart,
	}, nil
}

// startOfDay truncates a time to midnight in its own location so pinch dates land
// on a clean calendar day.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
