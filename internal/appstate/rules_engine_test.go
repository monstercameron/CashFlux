// SPDX-License-Identifier: MIT

package appstate_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// TestAutoCategorizeAppliesRename proves the C102 rename action fires on the
// ENTRY path — it previously applied only on the Apply-to-existing backfill,
// so a fresh transaction kept its raw description forever.
func TestAutoCategorizeAppliesRename(t *testing.T) {
	app := makeApp(t, domain.RoleOwner)
	acct := validAccount()
	if err := app.PutAccount(acct); err != nil {
		t.Fatalf("put account: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "cat-loan", Name: "Loans", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("put category: %v", err)
	}
	if err := app.PutRule(rules.Rule{
		ID: "r1", Match: "autopay loanco", SetCategoryID: "cat-loan",
		SetTags: []string{"loan"}, RenameDesc: "Auto loan payment",
	}); err != nil {
		t.Fatalf("put rule: %v", err)
	}

	got := app.AutoCategorizeTransaction(domain.Transaction{
		ID: "t1", AccountID: acct.ID, Date: time.Now(),
		Desc: "AUTOPAY LOANCO 0038-XK", Amount: money.Money{Amount: -25000, Currency: "USD"},
	})
	if got.CategoryID != "cat-loan" {
		t.Fatalf("category = %q, want cat-loan", got.CategoryID)
	}
	if got.Desc != "Auto loan payment" {
		t.Fatalf("rename on entry: desc = %q, want the rule's RenameDesc", got.Desc)
	}
}

// TestAutoLinkBillPayment proves a rule can carry ONLY a bill-account action (no
// category) and that it links a matching transaction as a bill payment — the path
// that auto-ties future/imported payments to the account the user first linked.
func TestAutoLinkBillPayment(t *testing.T) {
	app := makeApp(t, domain.RoleOwner)
	acct := validAccount()
	if err := app.PutAccount(acct); err != nil {
		t.Fatalf("put account: %v", err)
	}
	// A bill-only rule (no SetCategoryID) must be accepted (PutRule relaxation).
	if err := app.PutRule(rules.Rule{ID: "r-bill", Match: "rocket mortgage", SetBillAccountID: acct.ID}); err != nil {
		t.Fatalf("put bill rule: %v", err)
	}

	got := app.AutoCategorizeTransaction(domain.Transaction{
		ID: "t1", AccountID: "checking", Date: time.Now(),
		Desc: "ROCKET MORTGAGE #91002", Amount: money.Money{Amount: -148000, Currency: "USD"},
	})
	if got.BillAccountID != acct.ID {
		t.Fatalf("bill link = %q, want %q (auto-linked by rule)", got.BillAccountID, acct.ID)
	}

	// A transaction already linked by hand is never overwritten.
	manual := app.AutoCategorizeTransaction(domain.Transaction{
		ID: "t2", AccountID: "checking", Date: time.Now(), BillAccountID: "other-acct",
		Desc: "ROCKET MORTGAGE #91003", Amount: money.Money{Amount: -148000, Currency: "USD"},
	})
	if manual.BillAccountID != "other-acct" {
		t.Errorf("manual bill link overwritten: got %q, want other-acct", manual.BillAccountID)
	}
}

// TestPutRulePureConditions proves a rule may match by structured conditions
// alone — a match phrase is only required when there are no conditions.
func TestPutRulePureConditions(t *testing.T) {
	app := makeApp(t, domain.RoleOwner)
	cond := rules.Rule{
		ID: "r-cond", SetCategoryID: "cat-x",
		Conditions: []rules.RuleCondition{{Field: rules.ConditionFieldAmount, Op: rules.ConditionOpGt, Value: "10000"}},
	}
	if err := app.PutRule(cond); err != nil {
		t.Fatalf("pure-conditions rule rejected: %v", err)
	}
	if err := app.PutRule(rules.Rule{ID: "r-empty", SetCategoryID: "cat-x"}); err == nil {
		t.Fatalf("rule with no phrase AND no conditions should be rejected")
	}
}

// TestPreviewApplyRulesIsDry proves the blast-radius preview counts what the
// backfill would change without writing anything.
func TestPreviewApplyRulesIsDry(t *testing.T) {
	app := makeApp(t, domain.RoleOwner)
	acct := validAccount()
	if err := app.PutAccount(acct); err != nil {
		t.Fatalf("put account: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "cat-big", Name: "Big", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("put category: %v", err)
	}
	for i, amt := range []int64{-25000, -90000, -450} {
		txn := domain.Transaction{
			ID: "t" + string(rune('a'+i)), AccountID: acct.ID, Date: time.Now(),
			Desc: "store visit", Amount: money.Money{Amount: amt, Currency: "USD"},
		}
		if err := app.PutTransaction(txn); err != nil {
			t.Fatalf("put txn: %v", err)
		}
	}
	// Condition rule: outflows bigger than $100 (minor units < -10000).
	if err := app.PutRule(rules.Rule{
		ID: "r-big", SetCategoryID: "cat-big",
		Conditions: []rules.RuleCondition{{Field: rules.ConditionFieldAmount, Op: rules.ConditionOpLt, Value: "-10000"}},
	}); err != nil {
		t.Fatalf("put rule: %v", err)
	}

	total, perRule := app.PreviewApplyRules()
	if total != 2 || perRule["r-big"] != 2 {
		t.Fatalf("preview total=%d perRule=%v, want 2 / {r-big:2}", total, perRule)
	}
	// Dry: nothing was written.
	for _, txn := range app.Transactions() {
		if txn.CategoryID != "" {
			t.Fatalf("preview mutated transaction %s (category %q)", txn.ID, txn.CategoryID)
		}
	}
	// The real apply matches the preview.
	n, per, err := app.ApplyRulesWithCounts()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n != total || per["r-big"] != perRule["r-big"] {
		t.Fatalf("apply (n=%d per=%v) disagrees with preview (n=%d per=%v)", n, per, total, perRule)
	}
}

// TestNextRuleOrderAppends proves new rules land AFTER every existing rule in
// the first-match-wins chain (the zero Order + ID tie-break used to jump them
// to the top).
func TestNextRuleOrderAppends(t *testing.T) {
	app := makeApp(t, domain.RoleOwner)
	if err := app.PutRule(rules.Rule{ID: "rule-seed", Match: "coffee", SetCategoryID: "c1"}); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	next := app.NextRuleOrder()
	if next < 1 {
		t.Fatalf("NextRuleOrder = %d, want >= 1 with a seed rule present", next)
	}
	// A broad new rule added at the end must NOT shadow the seed.
	if err := app.PutRule(rules.Rule{ID: "aaaa-new", Match: "c", SetCategoryID: "c2", Order: next}); err != nil {
		t.Fatalf("new rule: %v", err)
	}
	rs := app.Rules()
	if len(rs) != 2 || rs[0].ID != "rule-seed" || rs[1].ID != "aaaa-new" {
		t.Fatalf("rule order = %v, want seed first then the appended rule", []string{rs[0].ID, rs[1].ID})
	}
}
