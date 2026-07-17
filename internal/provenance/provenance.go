// SPDX-License-Identifier: MIT

// Package provenance derives, for a headline figure, the plain facts behind it
// (#56): how many transactions were counted, across how many accounts, over
// which date range, and what was deliberately left out (transfers, rows
// excluded from reports). The UI's provenance popovers render these numbers so
// a figure is never a black box — rule 5, determinism & explainability.
//
// The counting rules deliberately MIRROR ledger.PeriodTotals (the function the
// masthead figures come from): a transaction counts when it is inside the
// half-open window [From, To), is not a transfer, and is not excluded from
// reports. Anything in-window that fails the last two tests is reported as
// left out rather than silently missing. Pure Go, no syscall/js, table-tested.
package provenance

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Flow describes the inputs behind a windowed cash-flow figure (income,
// spending, or the kept amount derived from both).
type Flow struct {
	// IncomeCount and ExpenseCount are the transactions that actually entered
	// the figure, split by direction.
	IncomeCount  int
	ExpenseCount int
	// TransferCount is the in-window transfers ignored by design (money moving
	// between accounts is not income or spending).
	TransferCount int
	// ExcludedCount is the in-window rows skipped because they are marked
	// "exclude from reports".
	ExcludedCount int
	// AccountCount is the live (non-archived) accounts in scope.
	AccountCount int
	// From and To bound the half-open window [From, To).
	From, To time.Time
}

// Counted is the total number of transactions that entered the figure.
func (f Flow) Counted() int { return f.IncomeCount + f.ExpenseCount }

// DescribeFlow computes the provenance of the window's cash-flow figures from
// an already-scoped transaction and account set.
func DescribeFlow(txns []domain.Transaction, accounts []domain.Account, from, to time.Time) Flow {
	f := Flow{From: from, To: to}
	for _, a := range accounts {
		if !a.Archived {
			f.AccountCount++
		}
	}
	for _, t := range txns {
		if t.Date.Before(from) || !t.Date.Before(to) {
			continue
		}
		switch {
		case t.IsTransfer():
			f.TransferCount++
		case !t.CountsInReports():
			f.ExcludedCount++
		case t.IsIncome():
			f.IncomeCount++
		case t.IsExpense():
			f.ExpenseCount++
		}
	}
	return f
}

// Balance describes the inputs behind a point-in-time balance figure (net
// worth): every transaction dated before the cutoff feeds it, across the live
// scoped accounts.
type Balance struct {
	// TxnCount is the transactions dated strictly before AsOf that feed the
	// balances (transfers included — they move balances even though they never
	// count as income or spending).
	TxnCount int
	// AccountCount is the live (non-archived) accounts whose balances sum into
	// the figure.
	AccountCount int
	// AsOf is the cutoff the balances are read at.
	AsOf time.Time
}

// DescribeBalance computes the provenance of a balances-as-of figure from an
// already-scoped transaction and account set.
func DescribeBalance(txns []domain.Transaction, accounts []domain.Account, asOf time.Time) Balance {
	b := Balance{AsOf: asOf}
	for _, a := range accounts {
		if !a.Archived {
			b.AccountCount++
		}
	}
	for _, t := range txns {
		if t.Date.Before(asOf) {
			b.TxnCount++
		}
	}
	return b
}
