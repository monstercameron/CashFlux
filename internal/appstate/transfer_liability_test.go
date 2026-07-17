// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// seedLiability creates a checking account plus a liability whose opening
// balance carries the given signed minor amount, exercising both at-rest debt
// conventions (the sample data stores debts negative; the "amount you owe" add
// form stores them positive).
func seedLiability(t *testing.T, a *App, openingMinor int64) (checking, loan domain.Account) {
	t.Helper()
	checking = domain.Account{
		ID: "chk1", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(100000, "USD"), // $1,000.00
	}
	loan = domain.Account{
		ID: "loan1", Name: "Loan", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD",
		OpeningBalance: money.New(openingMinor, "USD"),
	}
	if err := a.PutAccount(checking); err != nil {
		t.Fatalf("seedLiability PutAccount checking: %v", err)
	}
	if err := a.PutAccount(loan); err != nil {
		t.Fatalf("seedLiability PutAccount loan: %v", err)
	}
	return checking, loan
}

// balanceOf folds the account's booked balance and fails the test on error.
func balanceOf(t *testing.T, a *App, acc domain.Account) int64 {
	t.Helper()
	bal, err := ledger.Balance(acc, a.Transactions())
	if err != nil {
		t.Fatalf("ledger.Balance(%s): %v", acc.ID, err)
	}
	return bal.Amount
}

// TestTransferToPositiveStoredLiabilityReducesDebt reproduces the QA H1 defect:
// a $50 payment transfer into a $500 positive-stored loan must leave $450 owed,
// not $550.
func TestTransferToPositiveStoredLiabilityReducesDebt(t *testing.T) {
	a := newApp(t, false)
	checking, loan := seedLiability(t, a, 50000) // $500.00 owed, stored positive

	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: checking.ID, ToAccountID: loan.ID, AmountMinor: 5000,
	}); err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}

	if got := balanceOf(t, a, loan); got != 45000 {
		t.Errorf("loan balance after $50 payment = %d, want 45000 ($450 owed)", got)
	}
	if got := balanceOf(t, a, checking); got != 95000 {
		t.Errorf("checking balance after $50 payment = %d, want 95000", got)
	}
}

// TestTransferToNegativeStoredLiabilityReducesDebt covers the sample-data
// convention: a payment into a -$500 loan moves the balance to -$450.
func TestTransferToNegativeStoredLiabilityReducesDebt(t *testing.T) {
	a := newApp(t, false)
	checking, loan := seedLiability(t, a, -50000) // $500.00 owed, stored negative

	if _, _, err := a.CreateTransferPair(TransferParams{
		FromAccountID: checking.ID, ToAccountID: loan.ID, AmountMinor: 5000,
	}); err != nil {
		t.Fatalf("CreateTransferPair: %v", err)
	}

	if got := balanceOf(t, a, loan); got != -45000 {
		t.Errorf("loan balance after $50 payment = %d, want -45000 ($450 owed)", got)
	}
}

// TestBillPaymentToPositiveStoredLiabilityReducesDebt covers the same defect on
// the bills Mark-paid path (RecordBillPayment posts straight to the account).
func TestBillPaymentToPositiveStoredLiabilityReducesDebt(t *testing.T) {
	a := newApp(t, false)
	_, loan := seedLiability(t, a, 50000)

	if err := a.RecordBillPayment(loan.ID, "Loan", money.New(5000, "USD")); err != nil {
		t.Fatalf("RecordBillPayment: %v", err)
	}
	if got := balanceOf(t, a, loan); got != 45000 {
		t.Errorf("loan balance after $50 bill payment = %d, want 45000", got)
	}
}

// TestBillPaymentToNegativeStoredLiabilityStillWorks guards the previously
// correct sample-data path against regression.
func TestBillPaymentToNegativeStoredLiabilityStillWorks(t *testing.T) {
	a := newApp(t, false)
	_, loan := seedLiability(t, a, -50000)

	if err := a.RecordBillPayment(loan.ID, "Loan", money.New(5000, "USD")); err != nil {
		t.Fatalf("RecordBillPayment: %v", err)
	}
	if got := balanceOf(t, a, loan); got != -45000 {
		t.Errorf("loan balance after $50 bill payment = %d, want -45000", got)
	}
}
