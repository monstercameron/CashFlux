// SPDX-License-Identifier: MIT

package memberimpact

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestCompute(t *testing.T) {
	accounts := []domain.Account{
		{ID: "a1", Name: "Checking", OwnerID: "m1"},
		{ID: "a2", Name: "Joint Savings", OwnerID: domain.GroupOwnerID, OwnershipShares: map[string]int{"m1": 60, "m2": 40}},
		{ID: "a3", Name: "Her Card", OwnerID: "m2"},
	}
	budgets := []domain.Budget{
		{ID: "b1", Name: "Groceries", OwnerID: "m1"},
		{ID: "b2", Name: "Fun", OwnerID: domain.GroupOwnerID},
	}
	goals := []domain.Goal{
		{ID: "g1", Name: "Vacation", OwnerID: "m1"},
		{ID: "g2", Name: "Car", OwnerID: "m2"},
	}
	txns := []domain.Transaction{
		{ID: "t1", MemberID: "m1"},
		{ID: "t2", MemberID: "m2"},
		{ID: "t3", MemberID: "m1"},
		{ID: "t4"},
	}

	b := Compute("m1", accounts, budgets, goals, txns)
	if len(b.AccountsOwned) != 1 || b.AccountsOwned[0] != "Checking" {
		t.Errorf("AccountsOwned = %v, want [Checking]", b.AccountsOwned)
	}
	if len(b.AccountShares) != 1 || b.AccountShares[0] != "Joint Savings" {
		t.Errorf("AccountShares = %v, want [Joint Savings]", b.AccountShares)
	}
	if len(b.Budgets) != 1 || b.Budgets[0] != "Groceries" {
		t.Errorf("Budgets = %v, want [Groceries]", b.Budgets)
	}
	if len(b.Goals) != 1 || b.Goals[0] != "Vacation" {
		t.Errorf("Goals = %v, want [Vacation]", b.Goals)
	}
	if b.TxnCount != 2 {
		t.Errorf("TxnCount = %d, want 2", b.TxnCount)
	}
	if b.Total() != 6 || b.Empty() {
		t.Errorf("Total = %d Empty = %t, want 6 false", b.Total(), b.Empty())
	}

	// A directly-owned account must not double-count as a share.
	accounts[0].OwnershipShares = map[string]int{"m1": 100}
	b2 := Compute("m1", accounts, nil, nil, nil)
	if len(b2.AccountsOwned) != 1 || len(b2.AccountShares) != 1 {
		t.Errorf("owned+share split = %v / %v, want 1 owned (Checking) + 1 share (Joint Savings)", b2.AccountsOwned, b2.AccountShares)
	}

	// A member with nothing attached is Empty.
	if got := Compute("ghost", accounts, budgets, goals, txns); !got.Empty() {
		t.Errorf("ghost member breakdown = %+v, want empty", got)
	}
}
