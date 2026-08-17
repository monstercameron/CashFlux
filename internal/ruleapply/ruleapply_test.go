// SPDX-License-Identifier: MIT

package ruleapply

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func txn(id, payee, catID string, when time.Time) domain.Transaction {
	return domain.Transaction{
		ID: id, Payee: payee, Desc: payee, CategoryID: catID,
		Date: when, Amount: money.New(-1_000, "USD"),
	}
}

func coffeeRule() rules.Rule {
	return rules.Rule{ID: "r1", Match: "coffee", SetCategoryID: "cat-dining"}
}

func ledger() []domain.Transaction {
	return []domain.Transaction{
		txn("t1", "Coffee Shop", "", d(2024, time.March, 1)),             // uncategorized
		txn("t2", "Coffee Shop", "cat-groceries", d(2025, time.June, 1)), // wrongly filed
		txn("t3", "Coffee Shop", "cat-dining", d(2026, time.July, 1)),    // already right
		txn("t4", "Hardware Store", "cat-home", d(2026, time.July, 2)),   // not a match
	}
}

// The safe default. History is the part that cannot be un-seen, so a scope
// nobody chose must touch none of it.
func TestFutureScopeTouchesNoHistory(t *testing.T) {
	if got := Plan(coffeeRule(), ledger(), ScopeFuture, time.Time{}); len(got) != 0 {
		t.Errorf("future scope planned %d changes, want none", len(got))
	}
}

func TestAnUnknownScopeFallsBackToTheSafeOne(t *testing.T) {
	// If a stored preference is ever unreadable, the failure must be "did less
	// than expected", never "rewrote your history".
	if got := Plan(coffeeRule(), ledger(), Scope("nonsense"), time.Time{}); len(got) != 0 {
		t.Errorf("an unknown scope planned %d changes, want none", len(got))
	}
	if got := Scope("").Normalized(); got != ScopeFuture {
		t.Errorf("empty scope normalized to %q, want %q", got, ScopeFuture)
	}
}

func TestAllScopeChangesOnlyWhatActuallyDiffers(t *testing.T) {
	changes := Plan(coffeeRule(), ledger(), ScopeAll, time.Time{})
	if len(changes) != 2 {
		t.Fatalf("changes = %d, want 2 (the uncategorized one and the misfiled one)", len(changes))
	}
	byID := map[string]Change{}
	for _, c := range changes {
		byID[c.TxnID] = c
	}
	if _, ok := byID["t3"]; ok {
		t.Error("a transaction already in the target category is not an edit")
	}
	if _, ok := byID["t4"]; ok {
		t.Error("a non-matching transaction must not be touched")
	}
	if byID["t2"].FromCategoryID != "cat-groceries" {
		t.Errorf("t2 recorded from %q, want the category it actually had", byID["t2"].FromCategoryID)
	}
}

func TestSinceScopeStartsOnTheChosenDay(t *testing.T) {
	changes := Plan(coffeeRule(), ledger(), ScopeSince, d(2025, time.January, 1))
	if len(changes) != 1 || changes[0].TxnID != "t2" {
		t.Errorf("changes = %+v, want only t2 (2024 is before the cutoff)", changes)
	}
	// The cutoff day itself is included.
	onDay := Plan(coffeeRule(), ledger(), ScopeSince, d(2025, time.June, 1))
	if len(onDay) != 1 {
		t.Errorf("a transaction ON the cutoff must be included, got %d changes", len(onDay))
	}
}

func TestSinceWithNoDateTouchesNothing(t *testing.T) {
	// "From when?" unanswered is not "from the beginning of time".
	if got := Plan(coffeeRule(), ledger(), ScopeSince, time.Time{}); len(got) != 0 {
		t.Errorf("a since-scope with no date planned %d changes, want none", len(got))
	}
}

func TestUndoRestoresTheOldCategoryNotABlank(t *testing.T) {
	// The reason the old value is recorded at all: a transaction that was
	// already categorized has to go back to that category.
	changes := Plan(coffeeRule(), ledger(), ScopeAll, time.Time{})
	back := Undo(changes)
	if len(back) != len(changes) {
		t.Fatalf("undo produced %d changes, want %d", len(back), len(changes))
	}
	byID := map[string]Change{}
	for _, c := range back {
		byID[c.TxnID] = c
	}
	if byID["t2"].ToCategoryID != "cat-groceries" {
		t.Errorf("undo sends t2 to %q, want back to cat-groceries", byID["t2"].ToCategoryID)
	}
	if byID["t1"].ToCategoryID != "" {
		t.Errorf("undo sends t1 to %q, want back to uncategorized", byID["t1"].ToCategoryID)
	}
}

func TestUndoIsItsOwnInverse(t *testing.T) {
	changes := Plan(coffeeRule(), ledger(), ScopeAll, time.Time{})
	round := Undo(Undo(changes))
	for i := range changes {
		if round[i] != changes[i] {
			t.Fatalf("undoing twice changed row %d:\n got %+v\nwant %+v", i, round[i], changes[i])
		}
	}
}

func TestReclassifiedSeparatesOverwritesFromFills(t *testing.T) {
	// Filing 200 uncategorized transactions is the rule doing its job.
	// Overwriting 200 categories a person chose by hand is a different act.
	changes := Plan(coffeeRule(), ledger(), ScopeAll, time.Time{})
	if got := Reclassified(changes); got != 1 {
		t.Errorf("reclassified = %d, want 1 (only t2 had a category)", got)
	}
}

func TestOverBroadIgnoresUncategorizedRows(t *testing.T) {
	// Blank categories say nothing about what a rule means, and counting them
	// would flag every genuinely useful backfill.
	blanks := []Change{
		{TxnID: "a", ToCategoryID: "x"},
		{TxnID: "b", ToCategoryID: "x"},
		{TxnID: "c", ToCategoryID: "x"},
	}
	if n, broad := OverBroad(blanks); broad {
		t.Errorf("three blank rows read as %d categories and flagged over-broad", n)
	}
	spread := []Change{
		{TxnID: "a", FromCategoryID: "groceries", ToCategoryID: "x"},
		{TxnID: "b", FromCategoryID: "fuel", ToCategoryID: "x"},
		{TxnID: "c", FromCategoryID: "mortgage", ToCategoryID: "x"},
	}
	n, broad := OverBroad(spread)
	if !broad || n != 3 {
		t.Errorf("got %d categories (broad=%v), want 3 and flagged", n, broad)
	}
}

func TestTwoCategoriesIsOrdinaryNotOverBroad(t *testing.T) {
	// A payee that moved category once is normal, and warning about it would
	// teach people to ignore the warning.
	two := []Change{
		{TxnID: "a", FromCategoryID: "groceries", ToCategoryID: "x"},
		{TxnID: "b", FromCategoryID: "dining", ToCategoryID: "x"},
	}
	if _, broad := OverBroad(two); broad {
		t.Error("two existing categories must not be flagged as over-broad")
	}
}

func TestARuleWithNoCategoryPlansNothing(t *testing.T) {
	r := rules.Rule{ID: "r2", Match: "coffee", SetTags: []string{"treat"}}
	if got := Plan(r, ledger(), ScopeAll, time.Time{}); len(got) != 0 {
		t.Errorf("a tags-only rule planned %d category changes, want none", len(got))
	}
}

func TestThePlanIsStableAcrossRuns(t *testing.T) {
	// A bulk edit that shuffles between runs cannot be diffed, reviewed, or
	// trusted.
	for i := range 5 {
		a := Plan(coffeeRule(), ledger(), ScopeAll, time.Time{})
		b := Plan(coffeeRule(), ledger(), ScopeAll, time.Time{})
		for j := range a {
			if a[j] != b[j] {
				t.Fatalf("run %d row %d differed: %+v vs %+v", i, j, a[j], b[j])
			}
		}
	}
}
