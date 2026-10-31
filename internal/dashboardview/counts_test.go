// SPDX-License-Identifier: MIT

package dashboardview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCountPeriodTransactionsUsesReportAndHalfOpenPredicates(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	cases := []struct {
		name              string
		when              time.Time
		amount            int64
		exclude, transfer bool
		want              TransactionCounts
	}{
		{name: "start income", when: start, amount: 100, want: TransactionCounts{Income: 1}},
		{name: "last instant expense", when: end.Add(-time.Nanosecond), amount: -100, want: TransactionCounts{Expense: 1}},
		{name: "before start", when: start.Add(-time.Nanosecond), amount: 100},
		{name: "exclusive end", when: end, amount: -100},
		{name: "excluded income", when: start, amount: 100, exclude: true},
		{name: "excluded expense", when: start, amount: -100, exclude: true},
		{name: "transfer in", when: start, amount: 100, transfer: true},
		{name: "transfer out", when: start, amount: -100, transfer: true},
		{name: "zero", when: start},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			txn := domain.Transaction{ID: test.name, Date: test.when, Amount: money.New(test.amount, "USD"), ExcludeFromReports: test.exclude}
			if test.transfer {
				txn.TransferAccountID = "other"
			}
			if got := CountPeriodTransactions([]domain.Transaction{txn}, start, end); got != test.want {
				t.Fatalf("got %+v want %+v", got, test.want)
			}
		})
	}
}

func TestCountPeriodTransactionsDoesNotConvertOrMutateRows(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	rows := []domain.Transaction{{ID: "foreign", Date: start, Amount: money.New(42, "JPY")}, {ID: "expense", Date: start, Amount: money.New(-5, "EUR")}}
	before := append([]domain.Transaction(nil), rows...)
	if got := CountPeriodTransactions(rows, start, start.AddDate(0, 1, 0)); got != (TransactionCounts{Income: 1, Expense: 1}) {
		t.Fatalf("foreign rows count=%+v", got)
	}
	for i := range rows {
		if rows[i].Amount != before[i].Amount || rows[i].ExcludeFromReports != before[i].ExcludeFromReports {
			t.Fatal("counting changed source transactions")
		}
	}
	if got := CountPeriodTransactions(rows, start, start); got != (TransactionCounts{}) {
		t.Fatalf("empty window count=%+v", got)
	}
	if got := CountPeriodTransactions(nil, start, start.AddDate(0, 1, 0)); got != (TransactionCounts{}) {
		t.Fatalf("empty ledger count=%+v", got)
	}
}
