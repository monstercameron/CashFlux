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
