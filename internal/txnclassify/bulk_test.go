// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func bulkTxn(id, acct string, minor int64, catID string) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, CategoryID: catID,
		Amount: money.New(minor, "USD"),
		Date:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Desc:   "Transfer to Savings",
	}
}

func TestPlanBulkClassifiesEveryEligibleRow(t *testing.T) {
	rows := []domain.Transaction{
		bulkTxn("a", "chk", -30000, "c1"),
		bulkTxn("b", "chk", -20000, "c1"),
		bulkTxn("c", "chk", -10000, "c1"),
	}
	plan := PlanBulk(rows, "sav", false, pairAccounts())
	if len(plan.Applied) != 3 || len(plan.Failures) != 0 {
		t.Fatalf("applied=%d failures=%d, want 3/0", len(plan.Applied), len(plan.Failures))
	}
	if plan.LeavesTotalsMinor != 60000 {
		t.Errorf("leaves = %d, want 60000 — the size of the correction", plan.LeavesTotalsMinor)
	}
	for _, got := range plan.Applied {
		if got.TransferAccountID != "sav" {
			t.Errorf("%s counterparty = %q", got.ID, got.TransferAccountID)
		}
	}
}

// The acceptance is explicit: amount, date, sign and category are preserved.
func TestPlanBulkPreservesTheMoneyAndTheFiling(t *testing.T) {
	in := bulkTxn("a", "chk", -30000, "groceries")
	plan := PlanBulk([]domain.Transaction{in}, "sav", false, pairAccounts())
	got := plan.Applied[0]
	if got.Amount.Amount != in.Amount.Amount || got.Amount.Currency != in.Amount.Currency {
		t.Errorf("amount changed: %v -> %v", in.Amount, got.Amount)
	}
	if !got.Date.Equal(in.Date) {
		t.Errorf("date changed: %v -> %v", in.Date, got.Date)
	}
	if got.CategoryID != in.CategoryID {
		t.Errorf("category changed: %q -> %q", in.CategoryID, got.CategoryID)
	}
}

// A row already posted to the chosen account cannot be a transfer to itself. It
// must be REPORTED, not silently skipped: a bulk action that drops rows quietly
// has claimed work it did not do.
func TestPlanBulkReportsIncompatibleRowsByID(t *testing.T) {
	rows := []domain.Transaction{
		bulkTxn("ok", "chk", -30000, ""),
		bulkTxn("self", "sav", -20000, ""), // already on the target account
	}
	plan := PlanBulk(rows, "sav", false, pairAccounts())
	if len(plan.Applied) != 1 || plan.Applied[0].ID != "ok" {
		t.Fatalf("applied = %+v, want just 'ok'", plan.Applied)
	}
	if len(plan.Failures) != 1 || plan.Failures[0].TxnID != "self" {
		t.Fatalf("failures = %+v, want one for 'self'", plan.Failures)
	}
	if plan.Failures[0].Err == nil {
		t.Error("a failure carried no reason")
	}
}

// A mixed selection is the normal case. Asking for debt payments must not refuse
// the whole batch because the counterparty is an asset — the transfer part still
// applies, the claim simply does not.
func TestPlanBulkAppliesTheDebtClaimOnlyWhereItMeansSomething(t *testing.T) {
	rows := []domain.Transaction{bulkTxn("a", "chk", -30000, "")}

	toAsset := PlanBulk(rows, "sav", true, pairAccounts())
	if len(toAsset.Failures) != 0 {
		t.Fatalf("asked for a debt claim toward an asset and the batch failed: %+v", toAsset.Failures)
	}
	if toAsset.Applied[0].BillAccountID != "" || toAsset.DebtRows != 0 {
		t.Error("a payment intent was recorded against an asset account")
	}

	toCard := PlanBulk(rows, "card", true, pairAccounts())
	if len(toCard.Failures) != 0 {
		t.Fatalf("card batch failed: %+v", toCard.Failures)
	}
	if toCard.Applied[0].BillAccountID != "card" || toCard.DebtRows != 1 {
		t.Errorf("payment intent not recorded: bill=%q debtRows=%d",
			toCard.Applied[0].BillAccountID, toCard.DebtRows)
	}
}

// Nothing may be invented. The plan is exactly as long as the rows it was given.
func TestPlanBulkNeverAddsACounterpartRow(t *testing.T) {
	rows := []domain.Transaction{
		bulkTxn("a", "chk", -30000, ""),
		bulkTxn("b", "chk", -20000, ""),
	}
	plan := PlanBulk(rows, "sav", false, pairAccounts())
	if len(plan.Applied)+len(plan.Failures) != len(rows) {
		t.Errorf("plan covers %d rows for %d inputs — something was invented or dropped",
			len(plan.Applied)+len(plan.Failures), len(rows))
	}
}

// Re-running a batch that is already classified must not double-count the
// correction: those rows left the totals the first time.
func TestPlanBulkDoesNotCountAlreadyClassifiedRowsAsLeaving(t *testing.T) {
	already := bulkTxn("a", "chk", -30000, "")
	already.TransferAccountID = "sav"
	plan := PlanBulk([]domain.Transaction{already}, "sav", false, pairAccounts())
	if plan.LeavesTotalsMinor != 0 {
		t.Errorf("leaves = %d, want 0 — it was already out of the totals", plan.LeavesTotalsMinor)
	}
}

func TestEligibleSplitsOutRowsOnTheTargetAccount(t *testing.T) {
	rows := []domain.Transaction{
		bulkTxn("a", "chk", -30000, ""),
		bulkTxn("b", "sav", 30000, ""),
	}
	ok, blocked := Eligible(rows, "sav")
	if len(ok) != 1 || ok[0].ID != "a" {
		t.Errorf("eligible = %+v, want just 'a'", ok)
	}
	if len(blocked) != 1 || blocked[0].ID != "b" {
		t.Errorf("blocked = %+v, want just 'b'", blocked)
	}
}
