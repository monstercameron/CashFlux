// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func seedRuleTxn(t *testing.T, a *App, id, payee, cat string) {
	t.Helper()
	if err := a.PutTransaction(domain.Transaction{
		ID: id, AccountID: "chk", Payee: payee, Desc: payee,
		Amount: money.New(-1000, "USD"), Date: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		CategoryID: cat,
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
}

func TestApplyOneRuleHonoursPrecedence(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutAccount(domain.Account{ID: "chk", Name: "Chk", OwnerID: "m1",
		Scope: domain.ScopeIndividual, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	// Rule 1 (earlier, owns "coffee shop"); rule 2 (later, broader "shop").
	if err := a.PutRule(rules.Rule{ID: "r1", Match: "coffee shop", SetCategoryID: "cat-coffee", Order: 1}); err != nil {
		t.Fatalf("PutRule r1: %v", err)
	}
	if err := a.PutRule(rules.Rule{ID: "r2", Match: "shop", SetCategoryID: "cat-shopping", Order: 2}); err != nil {
		t.Fatalf("PutRule r2: %v", err)
	}
	seedRuleTxn(t, a, "t1", "Coffee Shop Downtown", "") // first-matched by r1
	seedRuleTxn(t, a, "t2", "Hardware Shop", "")        // first-matched by r2

	// Applying ONLY r2 must not steal t1 from r1.
	n, err := a.ApplyOneRule("r2")
	if err != nil {
		t.Fatalf("ApplyOneRule: %v", err)
	}
	if n != 1 {
		t.Errorf("applied %d, want 1", n)
	}
	got := map[string]string{}
	for _, tx := range a.Transactions() {
		got[tx.ID] = tx.CategoryID
	}
	if got["t1"] != "" {
		t.Errorf("t1 category = %q, want untouched (owned by r1)", got["t1"])
	}
	if got["t2"] != "cat-shopping" {
		t.Errorf("t2 category = %q, want cat-shopping", got["t2"])
	}

	// Unknown rule errors.
	if _, err := a.ApplyOneRule("nope"); err == nil {
		t.Error("unknown rule should error")
	}
}
