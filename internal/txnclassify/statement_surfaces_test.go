// SPDX-License-Identifier: MIT

package txnclassify

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func statementBounds() (time.Time, time.Time) {
	return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
}

// classifyImportedStatement models the review decision for every row in the
// supplied export. Rows whose description names the account they are already
// posted to are the export's duplicated/far legs; assigning them to savings is
// only a temporary counterparty so the classifier can be exercised. The account
// repair itself is deliberately tested separately below.
func classifyImportedStatement(t *testing.T) []domain.Transaction {
	t.Helper()
	accounts := statementAccounts()
	classified := make([]domain.Transaction, 0, len(statementTxns()))
	for _, original := range statementTxns() {
		counterparty := counterpartyFor(original.Desc, original.AccountID)
		if counterparty == "" {
			t.Fatalf("no counterparty parsed from %q", original.Desc)
		}
		if counterparty == original.AccountID {
			// This is not an account-repair operation. It keeps the row in the
			// classifier scenario while preserving the fact that Apply never
			// moves a transaction between accounts.
			counterparty = acctSavings
		}
		counterpartyAccount, ok := domain.AccountByID(accounts, counterparty)
		if !ok {
			t.Fatalf("counterparty %q not found for %q", counterparty, original.Desc)
		}
		claimDebt := counterpartyAccount.Class == domain.ClassLiability
		classifiedTxn, err := Apply(original, counterparty, claimDebt, accounts)
		if err != nil {
			t.Fatalf("Apply(%q): %v", original.Desc, err)
		}
		classified = append(classified, classifiedTxn)
	}
	return classified
}

// TestStatementSurfacesDropPhantomCashflowAfterReview is the end-to-end native
// scenario for the 40 supplied rows. Before review, the importer-shaped rows
// produce exactly the phantom income/spending observed in the private instance.
// After review, the same rows disappear from both the report and an ordinary
// spending budget, while the rows remain present for balance/debt accounting.
func TestStatementSurfacesDropPhantomCashflowAfterReview(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	start, end := statementBounds()
	original := statementTxns()
	if len(original) != 40 {
		t.Fatalf("fixture has %d rows, want 40", len(original))
	}

	before, err := reports.IncomeVsExpense(original, start, end, rates)
	if err != nil {
		t.Fatalf("report before review: %v", err)
	}
	if before.Income != 445000 || before.Expense != 2807786 {
		t.Fatalf("before review report = income %d, expense %d; want 445000 and 2807786",
			before.Income, before.Expense)
	}

	classified := classifyImportedStatement(t)
	if len(classified) != len(original) {
		t.Fatalf("classification changed row count from %d to %d", len(original), len(classified))
	}
	for _, txn := range classified {
		if !txn.IsTransfer() {
			t.Errorf("%q was not marked as a transfer", txn.Desc)
		}
	}

	after, err := reports.IncomeVsExpense(classified, start, end, rates)
	if err != nil {
		t.Fatalf("report after review: %v", err)
	}
	if after.Income != 0 || after.Expense != 0 {
		t.Errorf("after review report = income %d, expense %d; want both zero",
			after.Income, after.Expense)
	}

	spendingBudget := domain.Budget{
		ID:         "statement-spending",
		Name:       "Imported transfer rows",
		CategoryID: "cat-transfers",
		Period:     domain.PeriodYearly,
		Limit:      money.New(100000000, "USD"),
	}
	beforeSpent, err := budgeting.Spent(spendingBudget, original, start, end, rates)
	if err != nil {
		t.Fatalf("spending budget before review: %v", err)
	}
	if beforeSpent.Amount != 2807786 {
		t.Errorf("before review budget spend = %d; want 2807786", beforeSpent.Amount)
	}
	afterSpent, err := budgeting.Spent(spendingBudget, classified, start, end, rates)
	if err != nil {
		t.Fatalf("spending budget after review: %v", err)
	}
	if afterSpent.Amount != 0 {
		t.Errorf("after review spending budget = %d; want 0", afterSpent.Amount)
	}
}

// TestStatementDebtRowsKeepTheirPaymentMeaning checks the two liability
// destinations in the sample independently: *1677 is a card payment totalled
// at $18,609.38 and *1958 is a loan payment totalled at $4,500.00. Both are
// excluded from cashflow/spending, but retain BillAccountID for debt surfaces.
func TestStatementDebtRowsKeepTheirPaymentMeaning(t *testing.T) {
	classified := classifyImportedStatement(t)
	accounts := statementAccounts()
	var cardPayments, loanPayments int64
	var cardRows, loanRows int
	for _, txn := range classified {
		switch txn.TransferAccountID {
		case acct1677:
			cardRows++
			cardPayments += -txn.Amount.Amount
			if txn.BillAccountID != acct1677 || KindOf(txn, accounts) != KindDebt {
				t.Errorf("%q: card transfer lost its debt-payment meaning: bill=%q kind=%q",
					txn.Desc, txn.BillAccountID, KindOf(txn, accounts))
			}
		case acct1958:
			loanRows++
			loanPayments += -txn.Amount.Amount
			if txn.BillAccountID != acct1958 || KindOf(txn, accounts) != KindDebt {
				t.Errorf("%q: loan transfer lost its debt-payment meaning: bill=%q kind=%q",
					txn.Desc, txn.BillAccountID, KindOf(txn, accounts))
			}
		}
	}
	if cardRows != 15 || cardPayments != 1860938 {
		t.Errorf("*1677 debt rows/payments = %d/$%d; want 15/$1860938", cardRows, cardPayments)
	}
	if loanRows != 6 || loanPayments != 450000 {
		t.Errorf("*1958 debt rows/payments = %d/$%d; want 6/$450000", loanRows, loanPayments)
	}
}

// TestStatementMisfiledLegsStayVisibleForAccountRepair protects the boundary
// between classification and repair. The nine rows whose descriptions mention
// checking's own suffix are the duplicated/far legs and overstate checking by
// $2,950.00. Classification must not silently move their AccountID or invent a
// counterpart transaction; a later account-repair workflow must handle that.
func TestStatementMisfiledLegsStayVisibleForAccountRepair(t *testing.T) {
	original := statementTxns()
	classified := classifyImportedStatement(t)
	if len(classified) != len(original) {
		t.Fatalf("classification created/deleted rows: %d -> %d", len(original), len(classified))
	}

	var misfiled int
	var checkingNet int64
	for i, originalTxn := range original {
		if originalTxn.AccountID != acctChecking || !strings.Contains(originalTxn.Desc, "8945") {
			continue
		}
		misfiled++
		checkingNet += originalTxn.Amount.Amount
		if classified[i].AccountID != originalTxn.AccountID {
			t.Errorf("%q: classification moved AccountID from %q to %q",
				originalTxn.Desc, originalTxn.AccountID, classified[i].AccountID)
		}
	}
	if misfiled != 9 || checkingNet != 295000 {
		t.Errorf("self-named checking legs = %d, net = %d; want 9 and 295000", misfiled, checkingNet)
	}
}
