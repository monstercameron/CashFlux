// SPDX-License-Identifier: MIT

// Package resilience answers the anxious question a financial-health page exists
// for: "am I okay if something goes wrong?" Given a snapshot of monthly cash flow
// and buffers, it runs a small battery of what-if stress tests — losing income,
// an income cut, a surprise bill, a rate hike — and reports each outcome as a
// concrete figure (months of runway, when you'd go cash-negative, how much lands
// on the cards, the extra interest).
//
// Pure Go, no platform or i18n dependencies. Amounts are integer minor units of a
// single base currency; the caller FX-converts before calling. Rates are annual
// percents (e.g. 22.99).
package resilience

import "math"

// Input is the monthly cash-flow snapshot the stress tests run against. Living
// expenses and debt minimums are kept separate so a scenario can hold one while
// moving the other (e.g. income drops but the minimums don't).
type Input struct {
	LiquidCash       int64   // spendable buffer (cash-type accounts), minor units
	MonthlyIncome    int64   // typical monthly take-home, minor units
	MonthlySpend     int64   // typical monthly living expenses, EXCLUDING debt minimums
	MinDebtPayments  int64   // required debt minimums per month, minor units
	RevolvingBalance int64   // credit-card balances (what a rate hike bites), minor units
	AvgCardAPR       float64 // balance-weighted APR across the cards, annual percent
}

// totalOutflow is the full monthly outgo: living expenses plus debt minimums.
func (in Input) totalOutflow() int64 { return in.MonthlySpend + in.MinDebtPayments }

// MonthlySurplus is income minus every monthly outflow (may be negative).
func (in Input) MonthlySurplus() int64 { return in.MonthlyIncome - in.totalOutflow() }

// RunwayMonths is how many months the liquid buffer would cover the full monthly
// outflow with NO income at all — the headline resilience figure. Returns 0 when
// there's no outflow to cover (nothing to run out of) and is capped so a tiny
// outflow doesn't report centuries.
func RunwayMonths(in Input) float64 {
	out := in.totalOutflow()
	if out <= 0 {
		return 0
	}
	m := float64(in.LiquidCash) / float64(out)
	if m < 0 {
		m = 0
	}
	if m > 120 {
		m = 120 // 10 years — beyond this the precise number stops meaning anything
	}
	return m
}

// IncomeDropOutcome is the result of trimming income by some percent.
type IncomeDropOutcome struct {
	DropPct          int   // the cut applied, e.g. 20
	NewMonthlyIncome int64 // income after the cut
	NewSurplus       int64 // surplus after the cut (negative = burning the buffer)
	GoesNegative     bool  // true when the reduced income no longer covers outflow
	MonthsToNegative int   // whole months until the buffer is exhausted (only when GoesNegative)
}

// IncomeDrop models a sustained income cut of pct percent (0–100). When the reduced
// income can't cover the monthly outflow, it reports how many whole months the
// buffer would last before running dry.
func IncomeDrop(in Input, pct int) IncomeDropOutcome {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	newIncome := in.MonthlyIncome - int64(math.Round(float64(in.MonthlyIncome)*float64(pct)/100.0))
	surplus := newIncome - in.totalOutflow()
	out := IncomeDropOutcome{DropPct: pct, NewMonthlyIncome: newIncome, NewSurplus: surplus}
	if surplus < 0 {
		out.GoesNegative = true
		burn := -surplus
		if burn > 0 {
			out.MonthsToNegative = int(math.Floor(float64(in.LiquidCash) / float64(burn)))
		}
	}
	return out
}

// SurpriseOutcome is the result of an unplanned one-off expense.
type SurpriseOutcome struct {
	Amount               int64 // the surprise expense
	BufferAfter          int64 // liquid buffer after paying it (may be negative)
	PushedToDebt         int64 // the shortfall that would land on credit (0 when the buffer covers it)
	ExtraMonthlyInterest int64 // monthly interest on that shortfall at the card APR
}

// SurpriseExpense models an unplanned one-off bill of amount. If the buffer covers
// it, only the buffer shrinks; if it doesn't, the shortfall is assumed to go onto
// the cards, and the new monthly interest that shortfall accrues is reported.
func SurpriseExpense(in Input, amount int64) SurpriseOutcome {
	if amount < 0 {
		amount = 0
	}
	out := SurpriseOutcome{Amount: amount, BufferAfter: in.LiquidCash - amount}
	if out.BufferAfter < 0 {
		out.PushedToDebt = -out.BufferAfter
		apr := in.AvgCardAPR
		if apr < 0 {
			apr = 0
		}
		out.ExtraMonthlyInterest = int64(math.Round(float64(out.PushedToDebt) * apr / 1200.0))
	}
	return out
}

// RateHikeOutcome is the result of card APRs rising.
type RateHikeOutcome struct {
	Points               float64 // percentage points added to the APR
	ExtraMonthlyInterest int64   // additional interest per month on the current balance
	ExtraAnnualInterest  int64   // ... per year
}

// RateHike models the card APR rising by points percentage points, reporting the
// extra interest the current revolving balance would then accrue.
func RateHike(in Input, points float64) RateHikeOutcome {
	if points < 0 {
		points = 0
	}
	bal := in.RevolvingBalance
	if bal < 0 {
		bal = 0
	}
	monthly := int64(math.Round(float64(bal) * points / 1200.0))
	return RateHikeOutcome{Points: points, ExtraMonthlyInterest: monthly, ExtraAnnualInterest: monthly * 12}
}
