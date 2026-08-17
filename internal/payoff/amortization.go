// SPDX-License-Identifier: MIT

package payoff

import (
	"math"
	"time"
)

// AmortRow is one month's entry in a fixed-rate amortization schedule.
// All monetary fields are integer minor units (e.g. cents for USD).
// BalanceMinor is the remaining principal after this payment is applied.
type AmortRow struct {
	PaymentNo      int
	Date           time.Time
	PaymentMinor   int64
	PrincipalMinor int64
	InterestMinor  int64
	BalanceMinor   int64
}

// monthlyPayment computes the standard fixed-rate monthly payment M for a loan
// of principal P at monthly rate r over n payments, rounded to the nearest minor
// unit. When r == 0 (interest-free) it returns ceil(P/n) — the smallest uniform
// whole-unit payment that fully repays the balance within n months.
func monthlyPayment(principal int64, r float64, n int) int64 {
	if n <= 0 {
		return 0
	}
	if r == 0 {
		// Equal principal payments; use ceiling to avoid under-payment.
		return (principal + int64(n) - 1) / int64(n)
	}
	pow := math.Pow(1+r, float64(n))
	m := float64(principal) * r * pow / (pow - 1)
	return int64(math.Round(m))
}

// AmortizeFixed returns the full month-by-month amortization schedule for a
// fixed-rate installment loan. balanceMinor is the initial principal in minor
// units; aprPct is the annual percentage rate (e.g. 6.5 for 6.5%); termMonths
// is the number of scheduled payments; start is the date of the first payment.
//
// The monthly interest rate is aprPct/100/12. Each row's interest is
// round(balance * r) and its principal is payment − interest. The final payment
// is clamped so the ending balance is exactly 0 (mirroring the payoff engine's
// final-payment clamp).
//
// Returns nil when termMonths <= 0 (a minimum-payment simulation is out of
// scope for this function).
func AmortizeFixed(balanceMinor int64, aprPct float64, termMonths int, start time.Time) []AmortRow {
	if termMonths <= 0 {
		return nil
	}
	r := aprPct / 100.0 / 12.0
	payment := monthlyPayment(balanceMinor, r, termMonths)

	rows := make([]AmortRow, 0, termMonths)
	balance := balanceMinor
	date := start

	for i := 1; i <= termMonths && balance > 0; i++ {
		interest := int64(math.Round(float64(balance) * r))
		if interest < 0 {
			interest = 0
		}
		principal := payment - interest
		if principal < 0 {
			principal = 0
		}

		// Clamp the final payment so the balance lands exactly at 0.
		// This handles two cases: (a) the computed principal exceeds the remaining
		// balance (overshoot), and (b) we are on the last scheduled month but
		// integer rounding left a residual — both result in exact payoff.
		if principal > balance || i == termMonths {
			principal = balance
		}
		balance -= principal
		pay := principal + interest

		rows = append(rows, AmortRow{
			PaymentNo:      i,
			Date:           date,
			PaymentMinor:   pay,
			PrincipalMinor: principal,
			InterestMinor:  interest,
			BalanceMinor:   balance,
		})
		date = date.AddDate(0, 1, 0)
	}
	return rows
}

// AmortizeWithExtra returns a schedule like AmortizeFixed but applies an
// additional extraPerMonthMinor to the principal each month. This extra payment
// shortens the loan: the schedule stops as soon as the balance reaches 0, which
// will be earlier than termMonths whenever extra > 0. The final payment is
// clamped so the ending balance is exactly 0.
//
// Returns nil when termMonths <= 0.
func AmortizeWithExtra(balanceMinor int64, aprPct float64, termMonths int, extraPerMonthMinor int64, start time.Time) []AmortRow {
	if termMonths <= 0 {
		return nil
	}
	if extraPerMonthMinor < 0 {
		extraPerMonthMinor = 0
	}
	r := aprPct / 100.0 / 12.0
	basePayment := monthlyPayment(balanceMinor, r, termMonths)

	rows := make([]AmortRow, 0, termMonths)
	balance := balanceMinor
	date := start

	for i := 1; balance > 0; i++ {
		interest := int64(math.Round(float64(balance) * r))
		if interest < 0 {
			interest = 0
		}
		principal := basePayment - interest + extraPerMonthMinor
		if principal < 0 {
			principal = 0
		}

		// Clamp so we never pay more principal than remains.
		if principal > balance {
			principal = balance
		}
		balance -= principal
		pay := principal + interest

		rows = append(rows, AmortRow{
			PaymentNo:      i,
			Date:           date,
			PaymentMinor:   pay,
			PrincipalMinor: principal,
			InterestMinor:  interest,
			BalanceMinor:   balance,
		})
		date = date.AddDate(0, 1, 0)

		// Safety cap: never exceed the original term (in the degenerate case
		// where extra is negative-effective after the clamp).
		if i >= maxMonths {
			break
		}
	}
	return rows
}

// AmortSummary returns headline statistics for a completed amortization
// schedule: the total interest paid, the total amount paid (principal +
// interest), and the date of the last payment. If rows is empty, all values are
// their zero values and payoffDate is the zero time.Time.
func AmortSummary(rows []AmortRow) (totalInterestMinor, totalPaidMinor int64, payoffDate time.Time) {
	for _, row := range rows {
		totalInterestMinor += row.InterestMinor
		totalPaidMinor += row.PaymentMinor
		payoffDate = row.Date
	}
	return totalInterestMinor, totalPaidMinor, payoffDate
}

// RemainingMonths reports how many scheduled payments a loan has left at `now`,
// given its term and when it originated (C204).
//
// It counts WHOLE elapsed months from origination, so a loan originated on the
// 15th has used one payment on the following 15th, not on the 1st. The result is
// clamped to [0, termMonths]: a loan past its term has nothing left rather than a
// negative count, and a loan whose origination is in the future has its full term
// rather than more than it.
//
// ok is false when the inputs cannot answer the question — no term, or no
// origination date. Callers must treat that as "we do not know" and ask for the
// missing field, never as "zero months remain", which would render as a loan
// that is already paid off.
func RemainingMonths(termMonths int, origination, now time.Time) (int, bool) {
	if termMonths <= 0 || origination.IsZero() {
		return 0, false
	}
	elapsed := wholeMonthsBetween(origination, now)
	if elapsed < 0 {
		elapsed = 0
	}
	left := termMonths - elapsed
	if left < 0 {
		left = 0
	}
	return left, true
}

// PaymentsMade is RemainingMonths' complement: how many of the term's payments
// have come due. Clamped to [0, termMonths] for the same reasons.
func PaymentsMade(termMonths int, origination, now time.Time) (int, bool) {
	left, ok := RemainingMonths(termMonths, origination, now)
	if !ok {
		return 0, false
	}
	return termMonths - left, true
}

// wholeMonthsBetween counts complete calendar months from a to b. The day of
// month decides the boundary: Jan 15 → Feb 14 is zero whole months, Jan 15 →
// Feb 15 is one.
func wholeMonthsBetween(a, b time.Time) int {
	months := (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
	if b.Day() < a.Day() {
		months--
	}
	return months
}
