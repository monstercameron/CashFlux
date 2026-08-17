// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

// C676: a bulk classification is the widest write on the ledger, so it either
// happens or it does not. These pin both halves.

func bulkClassifyApp(t *testing.T) (*App, []domain.Transaction) {
	t.Helper()
	a := newApp(t, false)
	for _, acc := range []domain.Account{
		{ID: "a-check", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "a-save", Name: "Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD"},
	} {
		if err := a.PutAccount(acc); err != nil {
			t.Fatalf("PutAccount %s: %v", acc.ID, err)
		}
	}
	day := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	var rows []domain.Transaction
	for i, minor := range []int64{-50000, -25000, -12500} {
		tx := domain.Transaction{
			ID: "t" + string(rune('1'+i)), AccountID: "a-check", Desc: "TRANSFER TO SAVINGS",
			Date: day.AddDate(0, 0, i), Amount: money.New(minor, "USD"),
		}
		if err := a.PutTransaction(tx); err != nil {
			t.Fatalf("PutTransaction: %v", err)
		}
		rows = append(rows, tx)
	}
	return a, rows
}

func TestClassifyTransfersWritesEveryRow(t *testing.T) {
	a, rows := bulkClassifyApp(t)
	plan := txnclassify.PlanBulk(rows, "a-save", false, a.Accounts())
	if len(plan.Failures) != 0 {
		t.Fatalf("plan refused rows it should not: %+v", plan.Failures)
	}

	n, err := a.ClassifyTransfers(plan.Applied)
	if err != nil {
		t.Fatalf("ClassifyTransfers: %v", err)
	}
	if n != 3 {
		t.Errorf("wrote %d rows, want 3", n)
	}
	for _, got := range a.Transactions() {
		if !got.IsTransfer() {
			t.Errorf("row %s is still not a transfer", got.ID)
		}
		if got.TransferAccountID != "a-save" {
			t.Errorf("row %s points at %q, want a-save", got.ID, got.TransferAccountID)
		}
	}
}

// The classification must not touch the things a bulk action has no mandate over.
func TestClassifyTransfersLeavesAmountDateAndCategoryAlone(t *testing.T) {
	a, rows := bulkClassifyApp(t)
	for i := range rows {
		rows[i].CategoryID = "c-transfer"
	}
	plan := txnclassify.PlanBulk(rows, "a-save", false, a.Accounts())
	if _, err := a.ClassifyTransfers(plan.Applied); err != nil {
		t.Fatalf("ClassifyTransfers: %v", err)
	}
	before := map[string]domain.Transaction{}
	for _, r := range rows {
		before[r.ID] = r
	}
	for _, got := range a.Transactions() {
		was := before[got.ID]
		if got.Amount.Amount != was.Amount.Amount {
			t.Errorf("row %s amount changed %d → %d", got.ID, was.Amount.Amount, got.Amount.Amount)
		}
		if !got.Date.Equal(was.Date) {
			t.Errorf("row %s date changed %v → %v", got.ID, was.Date, got.Date)
		}
		if got.CategoryID != was.CategoryID {
			t.Errorf("row %s category changed %q → %q", got.ID, was.CategoryID, got.CategoryID)
		}
	}
}

// A bulk classification creates no counterpart rows: inventing the far side
// would be inventing money.
func TestClassifyTransfersCreatesNoRows(t *testing.T) {
	a, rows := bulkClassifyApp(t)
	before := len(a.Transactions())
	plan := txnclassify.PlanBulk(rows, "a-save", false, a.Accounts())
	if _, err := a.ClassifyTransfers(plan.Applied); err != nil {
		t.Fatalf("ClassifyTransfers: %v", err)
	}
	if after := len(a.Transactions()); after != before {
		t.Errorf("ledger went from %d rows to %d — a classification must not mint legs", before, after)
	}
}

// The whole point of the atomic write: a row the store refuses puts everything
// back, rather than leaving half a decision behind.
func TestClassifyTransfersRollsBackOnAFailedRow(t *testing.T) {
	a, rows := bulkClassifyApp(t)
	plan := txnclassify.PlanBulk(rows, "a-save", false, a.Accounts())
	if len(plan.Applied) != 3 {
		t.Fatalf("plan applied %d rows, want 3", len(plan.Applied))
	}
	// Corrupt the LAST row so the store refuses it after the first two are in.
	// An empty account id is a validation failure, not a classification one, so
	// it stands in for the store-level refusal this rollback exists for.
	poisoned := make([]domain.Transaction, len(plan.Applied))
	copy(poisoned, plan.Applied)
	poisoned[2].AccountID = ""

	n, err := a.ClassifyTransfers(poisoned)
	if err == nil {
		t.Fatal("a refused row did not fail the batch")
	}
	if n != 0 {
		t.Errorf("reported %d rows written after a rollback, want 0", n)
	}
	// Every row is back as it was: none classified.
	for _, got := range a.Transactions() {
		if got.IsTransfer() {
			t.Errorf("row %s stayed classified after the batch failed — half a decision was left behind", got.ID)
		}
	}
	if got := len(a.Transactions()); got != 3 {
		t.Errorf("ledger holds %d rows after rollback, want 3", got)
	}
}

// An empty plan is a no-op, not an error and not a write.
func TestClassifyTransfersEmptyPlan(t *testing.T) {
	a, _ := bulkClassifyApp(t)
	n, err := a.ClassifyTransfers(nil)
	if err != nil || n != 0 {
		t.Errorf("empty plan returned (%d, %v), want (0, nil)", n, err)
	}
}

// Review finding 3: a row that has been deleted from the store between the plan
// and the apply has no prior to restore. PutTransaction would CREATE it, and a
// rollback that only restored captured priors would leave a batch that "rolled
// back" having resurrected a transaction the ledger had deliberately lost.
func TestClassifyTransfersRemovesRowsItCreatedWhenRollingBack(t *testing.T) {
	a, rows := bulkClassifyApp(t)
	plan := txnclassify.PlanBulk(rows, "a-save", false, a.Accounts())

	// The first row is deleted under the surface after the plan was computed.
	if err := a.DeleteTransaction("t1"); err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}
	// The last row is then refused, so the batch must roll back — including the
	// row it just re-created.
	poisoned := make([]domain.Transaction, len(plan.Applied))
	copy(poisoned, plan.Applied)
	poisoned[2].AccountID = ""

	if _, err := a.ClassifyTransfers(poisoned); err == nil {
		t.Fatal("a refused row did not fail the batch")
	}
	for _, got := range a.Transactions() {
		if got.ID == "t1" {
			t.Error("the batch resurrected a row the ledger no longer had, and the rollback left it there")
		}
		if got.IsTransfer() {
			t.Errorf("row %s stayed classified after the batch failed", got.ID)
		}
	}
	if got := len(a.Transactions()); got != 2 {
		t.Errorf("ledger holds %d rows after rollback, want 2 — the deleted row must stay deleted", got)
	}
}
