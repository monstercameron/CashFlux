// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// ─── C549: merge into a category that does not exist yet ─────────────────────

// mergeApp is an unseeded app with one account to hang transactions off.
func mergeApp(t *testing.T) *App {
	t.Helper()
	a := newTestAppAt(t, time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC))
	putAccount(t, a, "acct", "Checking")
	return a
}

func putCat(t *testing.T, a *App, id, name string, kind domain.CategoryKind) {
	t.Helper()
	if err := a.PutCategory(domain.Category{ID: id, Name: name, Kind: kind}); err != nil {
		t.Fatalf("put category %s: %v", id, err)
	}
}

func putTxn(t *testing.T, a *App, id, catID string, minor int64) {
	t.Helper()
	if err := a.PutTransaction(domain.Transaction{
		ID: id, AccountID: "acct", CategoryID: catID, Desc: "test " + id,
		Date:   time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		Amount: money.New(minor, "USD"),
	}); err != nil {
		t.Fatalf("put transaction %s: %v", id, err)
	}
}

func TestMergeCategoriesIntoNewCreatesAndFolds(t *testing.T) {
	a := mergeApp(t)
	putCat(t, a, "c-gas1", "Gas", domain.KindExpense)
	putCat(t, a, "c-gas2", "Fuel", domain.KindExpense)
	putTxn(t, a, "t1", "c-gas1", -1000)
	putTxn(t, a, "t2", "c-gas2", -2000)

	newID, counts, err := a.MergeCategoriesIntoNew("Fuel & gas", "c-gas1", "c-gas2")
	if err != nil {
		t.Fatalf("MergeCategoriesIntoNew: %v", err)
	}
	if newID == "" {
		t.Fatal("no id for the created category")
	}
	// The count is the SUM across sources: a multi-source merge reporting only
	// the last source's numbers would understate what it did.
	if counts.Transactions != 2 {
		t.Errorf("Counts.Transactions = %d, want 2", counts.Transactions)
	}

	var found domain.Category
	for _, c := range a.Categories() {
		if c.ID == newID {
			found = c
		}
		if c.ID == "c-gas1" || c.ID == "c-gas2" {
			t.Errorf("source %q survived the merge", c.ID)
		}
	}
	if found.ID == "" {
		t.Fatal("the new category is not in the store")
	}
	if found.Name != "Fuel & gas" || found.Kind != domain.KindExpense {
		t.Errorf("new category = %+v, want name %q kind %q", found, "Fuel & gas", domain.KindExpense)
	}
	for _, tx := range a.Transactions() {
		if tx.CategoryID != newID {
			t.Errorf("transaction %s still files into %q", tx.ID, tx.CategoryID)
		}
	}
}

// A rejected name must fail BEFORE anything moves — creating the target first is
// what makes that true.
func TestMergeCategoriesIntoNewRefusesABadNameBeforeMoving(t *testing.T) {
	a := mergeApp(t)
	putCat(t, a, "c-a", "Dining", domain.KindExpense)
	putTxn(t, a, "t1", "c-a", -500)

	if _, _, err := a.MergeCategoriesIntoNew("   ", "c-a"); err == nil {
		t.Error("an empty name was accepted")
	}
	if _, _, err := a.MergeCategoriesIntoNew("Dining", "c-a"); err == nil {
		t.Error("a duplicate sibling name was accepted — PutCategory's uniqueness rule " +
			"must apply to a merge target too (C536/C537)")
	}
	if got := a.Transactions()[0].CategoryID; got != "c-a" {
		t.Errorf("the transaction was re-filed to %q despite the merge failing", got)
	}
}

// Mixing kinds is the data-integrity hazard the merge panel already refuses.
func TestMergeCategoriesIntoNewRefusesMixedKinds(t *testing.T) {
	a := mergeApp(t)
	putCat(t, a, "c-exp", "Dining", domain.KindExpense)
	putCat(t, a, "c-inc", "Salary", domain.KindIncome)

	if _, _, err := a.MergeCategoriesIntoNew("Mixed", "c-exp", "c-inc"); err == nil {
		t.Error("an expense and an income category were merged into one")
	}
}

// The same source named twice is one source, not two merges of a category that
// stops existing after the first.
func TestMergeCategoriesIntoNewDedupesSources(t *testing.T) {
	a := mergeApp(t)
	putCat(t, a, "c-a", "Dining", domain.KindExpense)
	putTxn(t, a, "t1", "c-a", -500)

	newID, counts, err := a.MergeCategoriesIntoNew("Eating out", "c-a", "c-a")
	if err != nil {
		t.Fatalf("MergeCategoriesIntoNew: %v", err)
	}
	if counts.Transactions != 1 {
		t.Errorf("Counts.Transactions = %d, want 1", counts.Transactions)
	}
	if got := a.Transactions()[0].CategoryID; got != newID {
		t.Errorf("transaction files into %q, want the new category", got)
	}
}

// ─── C372: the durable hit count is credited where work happens ──────────────

func TestApplyRulesCreditsHitsButPreviewDoesNot(t *testing.T) {
	a := mergeApp(t)
	putCat(t, a, "c-food", "Groceries", domain.KindExpense)
	if err := a.PutRule(rules.Rule{ID: "r1", Match: "market", SetCategoryID: "c-food"}); err != nil {
		t.Fatalf("put rule: %v", err)
	}
	for _, id := range []string{"t1", "t2"} {
		if err := a.PutTransaction(domain.Transaction{
			ID: id, AccountID: "acct", Desc: "Greenfield Market", Payee: "Greenfield Market",
			Date:   time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			Amount: money.New(-1500, "USD"),
		}); err != nil {
			t.Fatalf("put %s: %v", id, err)
		}
	}

	hits := func() (int, bool) {
		for _, r := range a.Rules() {
			if r.ID == "r1" {
				return r.HitCount, !r.LastRunAt.IsZero()
			}
		}
		t.Fatal("rule r1 vanished")
		return 0, false
	}

	// A dry run must not touch the record: a preview that inflated the count
	// would make the number mean nothing.
	a.PreviewApplyRules()
	if n, _ := hits(); n != 0 {
		t.Fatalf("PreviewApplyRules credited %d hits", n)
	}

	if _, _, err := a.ApplyRulesWithCounts(); err != nil {
		t.Fatalf("ApplyRulesWithCounts: %v", err)
	}
	n, dated := hits()
	if n != 2 {
		t.Errorf("HitCount = %d, want 2", n)
	}
	if !dated {
		t.Error("LastRunAt was not stamped")
	}

	// Re-running when there is nothing left to change credits nothing further —
	// the count is what the rule DID, not how often the backfill was pressed.
	if _, _, err := a.ApplyRulesWithCounts(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if n, _ := hits(); n != 2 {
		t.Errorf("HitCount = %d after a no-op re-run, want 2", n)
	}
}
