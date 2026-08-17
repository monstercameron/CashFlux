// SPDX-License-Identifier: MIT

// Package roundups is the pure logic for virtual round-up accrual (TX11).
//
// Each expense on a participating account "rounds up" to the next whole dollar;
// the accumulated spare change becomes a goal earmark on a weekly/monthly sweep.
// The accrual is VIRTUAL: it never mutates a transaction and never moves real
// money. It is a deterministic function over the transaction stream and the
// transaction-links (XC1/XC2), so it is fully explainable — every contributing
// transaction is listed — and unit-testable on native Go (no syscall/js).
package roundups

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/savings"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
)

// DollarMinor is the round-up granularity: the next whole dollar (100 cents).
const DollarMinor int64 = 100

// Contribution is one transaction's spare-change contribution to the jar, kept
// for the determinism/explainability rule (list the contributing txns).
type Contribution struct {
	TxnID      string
	Payee      string
	Date       time.Time
	SpendCents int64 // the expense magnitude in minor units
	RoundCents int64 // the spare change accrued (ceil-to-dollar minus spend)
}

// Accrual is the result of an accrual pass: the running jar total plus the
// contributing transactions and the currency the total is expressed in.
type Accrual struct {
	TotalCents    int64
	Contributions []Contribution
	Currency      string
}

// HasSpareChange reports whether anything accrued.
func (a Accrual) HasSpareChange() bool { return a.TotalCents > 0 }

// MinRateDays is how long round-ups must have been running before a rate can be
// projected from them (EC-13).
//
// Sixty days. "This goal finishes seven weeks sooner" is a promise, and a
// promise extrapolated from a fortnight of spare change is a fantasy — one
// holiday week of shopping would double the rate and halve the estimate.
const MinRateDays = 60

// MonthlyRateCents is the spare change this household actually accrues per
// month, and whether that is knowable yet.
//
// It is the OBSERVED rate rather than a theoretical one: how much rounded up in
// the window, scaled to a month. A rate nobody has sustained is not a rate, so
// too short a window and too few contributions both report false rather than a
// confident number — the honest failure for a figure whose whole job is to set
// an expectation.
func (a Accrual) MonthlyRateCents(days int) (int64, bool) {
	if days < MinRateDays || a.TotalCents <= 0 || len(a.Contributions) < MinRateDays/10 {
		return 0, false
	}
	return a.TotalCents * 30 / int64(days), true
}

// MonthsSooner is how much earlier a goal lands once the round-up rate is added
// to what is already being put aside.
//
// remainingCents is what is left to save and monthlyCents is the current
// contribution. It reports false when the answer would not be a fact: nothing
// being contributed at all (the goal has no date to bring forward), or a rate so
// small it changes nothing — "0 months sooner" dressed up as an accelerator is
// worse than saying nothing.
func MonthsSooner(remainingCents, monthlyCents, rateCents int64) (int, bool) {
	if remainingCents <= 0 || monthlyCents <= 0 || rateCents <= 0 {
		return 0, false
	}
	before := ceilDiv(remainingCents, monthlyCents)
	after := ceilDiv(remainingCents, monthlyCents+rateCents)
	if before <= after {
		return 0, false
	}
	return int(before - after), true
}

// ceilDiv rounds up, because a goal is not reached until the last partial month
// has actually happened.
func ceilDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// Accrue computes the virtual round-up jar for the window (since, now]:
//
//   - Only expenses count — income and transfers are skipped (transfers move
//     money you already had; they are not spending).
//   - Only participating accounts count. An empty participating set means "all
//     accounts participate", the friendly default before the user narrows it.
//   - Refund-paired transactions (XC2) are skipped on both sides: a refunded
//     purchase should not seed the jar with spare change that was handed back.
//   - Exact-dollar spends (round-up delta of 0) contribute nothing and are not
//     listed — there is no spare change to accrue.
//
// since is exclusive and now is inclusive, so a transaction dated exactly on the
// last sweep stamp is not double-counted by the next sweep.
func Accrue(txns []domain.Transaction, participating map[string]bool, links []domain.TxnLink, since, now time.Time) Accrual {
	out := Accrual{}
	for _, t := range txns {
		if !t.IsExpense() {
			continue
		}
		if len(participating) > 0 && !participating[t.AccountID] {
			continue
		}
		if !t.Date.After(since) || t.Date.After(now) {
			continue
		}
		if _, paired := txnlinks.PairOf(t.ID, links); paired {
			continue
		}
		spend := t.Amount.Amount
		if spend < 0 {
			spend = -spend
		}
		delta := savings.RoundUpDelta(spend, DollarMinor)
		if delta <= 0 {
			continue
		}
		if out.Currency == "" {
			out.Currency = t.Amount.Currency
		}
		out.TotalCents += delta
		out.Contributions = append(out.Contributions, Contribution{
			TxnID: t.ID, Payee: t.Payee, Date: t.Date,
			SpendCents: spend, RoundCents: delta,
		})
	}
	return out
}
