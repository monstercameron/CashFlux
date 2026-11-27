// SPDX-License-Identifier: MIT

package txnform

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBuildRejectsSimultaneousMoveAndSelfTransferAndHonorsUnclassifyDirection(t *testing.T) {
	row, accounts := example()

	// Moving plain row from "a" -> "b" while classifying it as transfer to "b" must be refused.
	d := Seed(row, true)
	d.AccountID = "b"
	d.TransferAccountID = "b"
	plan := Build(row, d, accounts, nil, time.Unix(10, 0))
	if plan.Err == nil || plan.Failure.Key != "transactions.accountMoveSelf" {
		t.Fatalf("simultaneous move+self-transfer succeeded: %+v", plan)
	}

	// Unclassifying a negative transfer and switching Direction to "in" must produce positive income.
	xrow := row
	xrow.TransferAccountID = "b"
	xrow.TransferGroupID = "g1"
	xrow.Amount = money.New(-5000, "USD")
	ud := Seed(xrow, true)
	ud.TransferAccountID = ""
	ud.Direction = "in"
	ud.Amount = "50.00"
	uplan := Build(xrow, ud, accounts, nil, time.Unix(10, 0))
	if uplan.Err != nil {
		t.Fatalf("unclassify build failed: %v", uplan.Err)
	}
	if uplan.Transaction.Amount.Amount != 5000 {
		t.Fatalf("unclassify direction in produced amount %d, want +5000", uplan.Transaction.Amount.Amount)
	}
	if uplan.Transaction.TransferGroupID != "" {
		t.Fatalf("unclassify left stale TransferGroupID=%q", uplan.Transaction.TransferGroupID)
	}
}

func TestBuildQuickSetsClearedAt(t *testing.T) {
	s := quickTestSource()
	d := NewQuickDraft("")
	d.Amount, d.Direction, d.Payee, d.Cleared = "10.00", "Expense", "Cafe", true
	txn, err := BuildQuick(d, s.Accounts, "usd", "", "q1", s.Now)
	if err != nil {
		t.Fatalf("BuildQuick: %v", err)
	}
	if !txn.Cleared || txn.ClearedAt.IsZero() {
		t.Fatalf("cleared quick txn missing ClearedAt: %+v", txn)
	}
}
