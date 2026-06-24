// SPDX-License-Identifier: MIT

// Package payoff computes debt-payoff projections: given a balance owed, an
// annual interest rate, and a fixed monthly payment, it simulates the month-by-
// month paydown and reports how long it takes and how much interest accrues.
//
// Amounts are integer minor units (e.g. cents); rates are annual percentages
// (e.g. 19.99 for 19.99% APR). Pure Go, no platform dependencies; unit-tested on
// native Go. The planning UI calls into here.
package payoff

import "math"

// maxMonths caps the simulation so an underpaid balance can't loop forever.
const maxMonths = 1200 // 100 years

// Result summarizes a payoff projection.
type Result struct {
	Months        int   // months until the balance reaches zero
	TotalInterest int64 // total interest accrued over the payoff, in minor units
	TotalPaid     int64 // principal + interest paid, in minor units
}

// MinimumViablePayment returns the smallest whole-minor-unit monthly payment that
// will eventually clear the balance at aprPercent: one minor unit more than the
// first month's interest. Because the balance only falls, the first month's
// interest is the largest, so any payment above it reduces principal every month
// and the debt clears. A non-positive balance returns 0 (nothing owed); a
// non-positive APR returns 1 (any positive payment clears it).
func MinimumViablePayment(balance int64, aprPercent float64) int64 {
	if balance <= 0 {
		return 0
	}
	if aprPercent <= 0 {
		return 1
	}
	interest := int64(math.Round(float64(balance) * (aprPercent / 1200.0)))
	if interest < 0 {
		interest = 0
	}
	return interest + 1
}

// Project simulates paying down balance (minor units, a positive amount owed) at
// aprPercent annual interest with a fixed payment each month (minor units).
// Interest compounds monthly at apr/12 and is rounded to the nearest minor unit.
//
// It returns ok=false when the payment can never cover the monthly interest (so
// the balance would never fall) — except a zero/negative balance is already paid
// (ok=true, zero result). A zero or negative payment on a positive balance is not
// viable (ok=false).
func Project(balance int64, aprPercent float64, payment int64) (Result, bool) {
	if balance <= 0 {
		return Result{}, true
	}
	if payment <= 0 {
		return Result{}, false
	}

	monthlyRate := aprPercent / 1200.0 // percent → fraction, annual → monthly
	orig := balance
	var totalInterest int64
	months := 0

	for balance > 0 && months < maxMonths {
		interest := int64(math.Round(float64(balance) * monthlyRate))
		if interest < 0 {
			interest = 0
		}
		// If the payment can't cover the interest, the balance never falls.
		if payment <= interest {
			return Result{}, false
		}
		balance += interest
		totalInterest += interest

		pay := payment
		if pay > balance {
			pay = balance
		}
		balance -= pay
		months++
	}

	if balance > 0 {
		return Result{}, false
	}
	return Result{Months: months, TotalInterest: totalInterest, TotalPaid: orig + totalInterest}, true
}
