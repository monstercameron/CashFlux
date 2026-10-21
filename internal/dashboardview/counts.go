// SPDX-License-Identifier: MIT

package dashboardview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// TransactionCounts contains report-eligible positive and negative rows. Counts
// describe transactions, independent of currency conversion or refund amounts.
type TransactionCounts struct{ Income, Expense int }

// CountPeriodTransactions counts an already-scoped ledger over [start,end),
// using the same report inclusion and transfer/sign predicates as headline totals.
func CountPeriodTransactions(transactions []domain.Transaction, start, end time.Time) TransactionCounts {
	var counts TransactionCounts
	for _, transaction := range transactions {
		if !transaction.CountsInReports() || !dateutil.InRange(transaction.Date, start, end) {
			continue
		}
		switch {
		case transaction.IsIncome():
			counts.Income++
		case transaction.IsExpense():
			counts.Expense++
		}
	}
	return counts
}
