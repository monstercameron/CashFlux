// SPDX-License-Identifier: MIT

package provenance

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(d int) time.Time { return time.Date(2026, time.July, d, 0, 0, 0, 0, time.UTC) }

func txn(d int, amountMinor int64) domain.Transaction {
	return domain.Transaction{Date: day(d), Amount: money.New(amountMinor, "USD"), AccountID: "a1"}
}

func TestDescribeFlow(t *testing.T) {
	from, to := day(1), day(31) // [Jul 1, Jul 31)
	transfer := txn(5, -2000)
	transfer.TransferAccountID = "a2"
	excluded := txn(6, -1500)
	excluded.ExcludeFromReports = true

	txns := []domain.Transaction{
		txn(2, 100000),  // income, counted
		txn(3, -4000),   // expense, counted
		txn(10, -2500),  // expense, counted
		transfer,        // ignored by design
		excluded,        // excluded from reports
		txn(31, -9900),  // on To — outside the half-open window
		txn(30, -1200),  // last in-window day, counted
		{Date: day(15)}, // zero amount: neither income nor expense — not counted
	}
	accounts := []domain.Account{
		{ID: "a1"}, {ID: "a2"}, {ID: "a3", Archived: true},
	}

	f := DescribeFlow(txns, accounts, from, to)
	if f.IncomeCount != 1 || f.ExpenseCount != 3 {
		t.Errorf("counted income=%d expense=%d, want 1/3", f.IncomeCount, f.ExpenseCount)
	}
	if f.Counted() != 4 {
		t.Errorf("Counted() = %d, want 4", f.Counted())
	}
	if f.TransferCount != 1 {
		t.Errorf("TransferCount = %d, want 1", f.TransferCount)
	}
	if f.ExcludedCount != 1 {
		t.Errorf("ExcludedCount = %d, want 1", f.ExcludedCount)
	}
	if f.AccountCount != 2 {
		t.Errorf("AccountCount = %d, want 2 (archived never counts)", f.AccountCount)
	}
}

func TestDescribeFlowExcludedTransferCountsAsTransfer(t *testing.T) {
	// A transfer that is ALSO marked excluded reads as a transfer — the
	// stronger, by-design reason it never counts.
	tr := txn(5, -2000)
	tr.TransferAccountID = "a2"
	tr.ExcludeFromReports = true
	f := DescribeFlow([]domain.Transaction{tr}, nil, day(1), day(31))
	if f.TransferCount != 1 || f.ExcludedCount != 0 {
		t.Errorf("transfer+excluded → transfers=%d excluded=%d, want 1/0", f.TransferCount, f.ExcludedCount)
	}
}

func TestDescribeBalance(t *testing.T) {
	txns := []domain.Transaction{
		txn(2, 100000),
		txn(30, -1200),
		txn(31, -500), // dated on the cutoff — balances read strictly before it
	}
	// Transfers feed balances too.
	tr := txn(10, -2000)
	tr.TransferAccountID = "a2"
	txns = append(txns, tr)

	b := DescribeBalance(txns, []domain.Account{{ID: "a1"}, {ID: "a2", Archived: true}}, day(31))
	if b.TxnCount != 3 {
		t.Errorf("TxnCount = %d, want 3 (cutoff-day row and nothing archived-related dropped)", b.TxnCount)
	}
	if b.AccountCount != 1 {
		t.Errorf("AccountCount = %d, want 1", b.AccountCount)
	}
}
