// SPDX-License-Identifier: MIT

package smartengine

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestA9FeeBleed(t *testing.T) {
	in := baseInput()
	old := ref.AddDate(0, -10, 0) // last real activity 10 months ago
	acc := acct("a1", "Old Brokerage Cash", domain.TypeSavings, 100000, old)
	// Monthly $5 fee for the last 4 months → otherwise-dormant fee bleed.
	var txns []domain.Transaction
	for i := 1; i <= 4; i++ {
		txns = append(txns, txn("f"+string(rune('0'+i)), "a1", ref.AddDate(0, -i, 0), -500))
	}
	in.Accounts = []domain.Account{acc}
	in.Transactions = txns

	got := a9FeeBleed(in)
	ins, ok := findInsight(got, "SMART-A9")
	if !ok {
		t.Fatal("expected a SMART-A9 fee-bleed insight")
	}
	if ins.Key != "SMART-A9:a1" {
		t.Errorf("key = %q, want SMART-A9:a1", ins.Key)
	}
	if ins.Action == nil || ins.Action.Kind != smart.ActionCreateTask || ins.Action.RelatedID != "a1" {
		t.Errorf("expected a create-task action tied to the account, got %+v", ins.Action)
	}
}

func TestA9NoFlagWithRealActivity(t *testing.T) {
	in := baseInput()
	acc := acct("a1", "Active", domain.TypeChecking, 100000, ref)
	in.Accounts = []domain.Account{acc}
	in.Transactions = []domain.Transaction{
		txn("f1", "a1", ref.AddDate(0, -1, 0), -500),   // a fee
		txn("d1", "a1", ref.AddDate(0, -1, 0), 200000), // but also a real deposit → not dormant
	}
	if _, ok := findInsight(a9FeeBleed(in), "SMART-A9"); ok {
		t.Error("account with real activity should not be flagged as fee-bleed")
	}
}

func TestA9NoFlagLargeDebit(t *testing.T) {
	in := baseInput()
	acc := acct("a1", "Spender", domain.TypeChecking, 100000, ref)
	in.Accounts = []domain.Account{acc}
	in.Transactions = []domain.Transaction{
		txn("f1", "a1", ref.AddDate(0, -1, 0), -500),  // a fee
		txn("b1", "a1", ref.AddDate(0, -2, 0), -8000), // a real $80 purchase → not just fees
	}
	if _, ok := findInsight(a9FeeBleed(in), "SMART-A9"); ok {
		t.Error("account with a non-trivial debit should not be flagged as fee-bleed")
	}
}
