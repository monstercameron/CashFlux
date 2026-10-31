// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func txnWithRuleApp(t *testing.T) (*App, domain.Transaction) {
	t.Helper()
	a := newApp(t, false)
	t.Cleanup(func() { _ = a.Store().Close() })
	if err := a.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	txn := domain.Transaction{ID: "pay1", AccountID: "bank", Desc: "Electric Co",
		Date: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), Amount: money.New(-4500, "USD")}
	if err := a.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	stored, ok := a.TransactionByID(txn.ID)
	if !ok {
		t.Fatal("seed transaction missing")
	}
	return a, stored
}

func TestPutTransactionWithRuleRejectedRuleLeavesTransactionUntouched(t *testing.T) {
	a, before := txnWithRuleApp(t)
	edited := before
	edited.BillAccountID = "bank"
	calls := 0
	a.OnTxnMutated(func() { calls++ })
	rev := a.Rev()
	cursor, err := a.Store().ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	for name, rule := range map[string]rules.Rule{
		"no id":     {Match: "electric", SetBillAccountID: "bank"},
		"no match":  {ID: "r1", SetBillAccountID: "bank"},
		"no action": {ID: "r1", Match: "electric"},
	} {
		if err := a.PutTransactionWithRule(edited, rule); err == nil {
			t.Fatalf("%s: invalid rule was accepted", name)
		}
	}
	got, _ := a.TransactionByID(before.ID)
	nextCursor, cursorErr := a.Store().ChangeCursor()
	if !reflect.DeepEqual(got, before) || len(a.Rules()) != 0 || calls != 0 || a.Rev() != rev || cursorErr != nil || nextCursor != cursor {
		t.Fatalf("rejected rule changed state: txn=%+v rules=%d notifications=%d", got, len(a.Rules()), calls)
	}
}

func TestPutTransactionWithRuleWritesBoth(t *testing.T) {
	a, before := txnWithRuleApp(t)
	edited := before
	edited.BillAccountID = "bank"
	rule := rules.Rule{ID: "r1", Match: "electric", SetBillAccountID: "bank"}
	calls := 0
	a.OnTxnMutated(func() { calls++ })
	if err := a.PutTransactionWithRule(edited, rule); err != nil {
		t.Fatal(err)
	}
	got, _ := a.TransactionByID(before.ID)
	stored := a.Rules()
	if got.BillAccountID != "bank" || len(stored) != 1 || stored[0].ID != "r1" || calls != 1 {
		t.Fatalf("commit incomplete: txn=%+v rules=%+v notifications=%d", got, stored, calls)
	}
}

func TestPutTransactionWithRuleRejectedTransactionStoresNoRule(t *testing.T) {
	a, before := txnWithRuleApp(t)
	bad := before
	bad.AccountID = "missing"
	if err := a.PutTransactionWithRule(bad, rules.Rule{ID: "r1", Match: "electric", SetBillAccountID: "bank"}); err == nil {
		t.Fatal("invalid transaction was accepted")
	}
	if got, _ := a.TransactionByID(before.ID); !reflect.DeepEqual(got, before) || len(a.Rules()) != 0 {
		t.Fatalf("rejected transaction left a partial save: txn=%+v rules=%d", got, len(a.Rules()))
	}
}
