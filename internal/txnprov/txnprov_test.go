// SPDX-License-Identifier: MIT

package txnprov_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/txnprov"
)

func txn(mut func(*domain.Transaction)) domain.Transaction {
	t := domain.Transaction{
		ID: "t1", AccountID: "a1", Payee: "SQ *BLUE BOTTLE #47", Desc: "Coffee",
		CategoryID: "c-dining", Amount: money.Money{Amount: -675, Currency: "USD"},
		Date: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Source: domain.TxnSourceImported,
	}
	if mut != nil {
		mut(&t)
	}
	return t
}

var diningRule = rules.Rule{ID: "r1", Match: "blue bottle", SetCategoryID: "c-dining"}

func TestUncategorizedHasNothingToAttribute(t *testing.T) {
	got := txnprov.Of(txn(func(x *domain.Transaction) { x.CategoryID = "" }), []rules.Rule{diningRule})
	if got.Level != txnprov.None {
		t.Errorf("Level = %v, want None", got.Level)
	}
	if got.RuleMatch != nil {
		t.Error("named a rule for a row with no category")
	}
}

// A transfer leg carries no category by design. Marking it "automatic" would
// invent a decision nobody made and put a badge on rows that can never be fixed.
func TestTransferLegIsNotAttributed(t *testing.T) {
	got := txnprov.Of(txn(func(x *domain.Transaction) { x.TransferAccountID = "a2" }), []rules.Rule{diningRule})
	if got.Level != txnprov.None {
		t.Errorf("Level = %v, want None for a transfer leg", got.Level)
	}
}

// A split row shows its SPLIT LINES' categories, while CategoryID still holds the
// single category it carried before the split. Judging the row by that field puts
// a mark — and a rule name — on a category that appears nowhere on the row.
func TestSplitRowIsNotAttributedThroughItsStaleTopCategory(t *testing.T) {
	split := txn(func(x *domain.Transaction) {
		x.CategoryID = "c-dining" // stale: what it was before the split
		x.Splits = []domain.CategorySplit{
			{CategoryID: "c-groceries", Amount: money.Money{Amount: -400, Currency: "USD"}},
			{CategoryID: "c-travel", Amount: money.Money{Amount: -275, Currency: "USD"}},
		}
	})
	got := txnprov.Of(split, []rules.Rule{diningRule})
	if got.Level != txnprov.None {
		t.Errorf("Level = %v, want None for a split row", got.Level)
	}
	if got.RuleMatch != nil {
		t.Errorf("named rule %v for a split row — that rule describes a category the row does not show", got.RuleMatch)
	}
}

// Confirming is the act that transfers ownership of the decision from the machine
// to the person. A rule that happens to agree must not pull it back.
func TestReviewedOutranksAMatchingRule(t *testing.T) {
	got := txnprov.Of(txn(func(x *domain.Transaction) { x.Reviewed = true }), []rules.Rule{diningRule})
	if got.Level != txnprov.Confirmed {
		t.Errorf("Level = %v, want Confirmed", got.Level)
	}
	if got.RuleMatch != nil {
		t.Error("attributed a confirmed row to a rule")
	}
}

func TestManualEntryIsItsOwnConfirmation(t *testing.T) {
	got := txnprov.Of(txn(func(x *domain.Transaction) { x.Source = domain.TxnSourceManual }), []rules.Rule{diningRule})
	if got.Level != txnprov.Confirmed {
		t.Errorf("Level = %v, want Confirmed for a hand-entered row", got.Level)
	}
}

func TestImportedRowIsAutomaticAndNamesTheRuleThatExplainsIt(t *testing.T) {
	got := txnprov.Of(txn(nil), []rules.Rule{diningRule})
	if got.Level != txnprov.Automatic {
		t.Fatalf("Level = %v, want Automatic", got.Level)
	}
	if got.RuleMatch == nil || got.RuleMatch.ID != "r1" {
		t.Fatalf("RuleMatch = %v, want the dining rule", got.RuleMatch)
	}
	if !got.IsAutomatic() {
		t.Error("IsAutomatic() disagreed with Level")
	}
}

// The row is still automatic when nothing currently explains it — an old import,
// or a rule that has since been edited. Saying "automatic, source unknown" is
// honest; silently promoting it to Confirmed is not.
func TestAutomaticWithNoExplainingRule(t *testing.T) {
	got := txnprov.Of(txn(nil), nil)
	if got.Level != txnprov.Automatic {
		t.Errorf("Level = %v, want Automatic", got.Level)
	}
	if got.RuleMatch != nil {
		t.Error("named a rule when there were none")
	}
}

// The trap this guards: taking the first MATCHING rule would name a rule that
// files the row somewhere else, telling the user their Dining charge "came from"
// a rule that would have made it Groceries.
func TestARuleThatWouldFileItElsewhereIsNotTheExplanation(t *testing.T) {
	elsewhere := rules.Rule{ID: "r-first", Match: "blue bottle", SetCategoryID: "c-groceries"}
	got := txnprov.Of(txn(nil), []rules.Rule{elsewhere, diningRule})
	if got.Level != txnprov.Automatic {
		t.Fatalf("Level = %v, want Automatic", got.Level)
	}
	if got.RuleMatch == nil || got.RuleMatch.ID != "r1" {
		t.Fatalf("RuleMatch = %v, want the rule whose category matches the row", got.RuleMatch)
	}
}

// A tag-only or rename-only rule sets no category, so it explains nothing about
// where the row is filed.
func TestARuleThatSetsNoCategoryExplainsNothing(t *testing.T) {
	tagOnly := rules.Rule{ID: "r-tags", Match: "blue bottle", SetTags: []string{"coffee"}}
	got := txnprov.Of(txn(nil), []rules.Rule{tagOnly})
	if got.RuleMatch != nil {
		t.Errorf("RuleMatch = %v, want nil for a tag-only rule", got.RuleMatch)
	}
}

// Without this, a user who opens an "auto" row, picks a category by hand and
// saves would watch the row keep telling them a machine chose it.
func TestConfirmsCategory(t *testing.T) {
	cases := []struct {
		name          string
		before, after string
		want          bool
	}{
		{"picked a category where a rule had guessed", "c-groceries", "c-dining", true},
		{"filed a previously uncategorized row", "", "c-dining", true},
		{"saved without touching the category", "c-dining", "c-dining", false},
		{"cleared the category", "c-dining", "", false},
		{"saved an uncategorized row unchanged", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := txnprov.ConfirmsCategory(
				txn(func(x *domain.Transaction) { x.CategoryID = tc.before }),
				txn(func(x *domain.Transaction) { x.CategoryID = tc.after }),
			)
			if got != tc.want {
				t.Errorf("ConfirmsCategory(%q -> %q) = %v, want %v", tc.before, tc.after, got, tc.want)
			}
		})
	}
}

// The whole point of confirming: the row stops reading as automatic.
func TestConfirmingClearsTheAutomaticMark(t *testing.T) {
	row := txn(nil)
	if !txnprov.Of(row, []rules.Rule{diningRule}).IsAutomatic() {
		t.Fatal("fixture is not automatic to begin with")
	}
	edited := row
	edited.CategoryID = "c-groceries"
	if txnprov.ConfirmsCategory(row, edited) {
		edited.Reviewed = true
	}
	if txnprov.Of(edited, []rules.Rule{diningRule}).IsAutomatic() {
		t.Error("a hand-picked category still reads as automatic")
	}
}

// Rules written against the tidy merchant name must still explain a row whose
// stored payee is the processor descriptor — that is how the engine matches
// (ruleMatchesFull searches the cleaned payee too), and provenance has to agree
// with it or the row's badge and the Rules page tell different stories.
// "TST* BLUEBOTTLE 1234" cleans to "Bluebottle", which the raw string does not
// contain, so this only passes if the cleaned form is really being searched.
func TestExplanationFollowsTheEngineOnCleanedPayees(t *testing.T) {
	row := txn(func(x *domain.Transaction) { x.Payee = "TST* BLUEBOTTLE 1234"; x.Desc = "" })
	got := txnprov.Of(row, []rules.Rule{{ID: "r-clean", Match: "Bluebottle", SetCategoryID: "c-dining"}})
	if got.RuleMatch == nil || got.RuleMatch.ID != "r-clean" {
		t.Fatalf("RuleMatch = %v — a rule written against the cleaned merchant name did not explain the row", got.RuleMatch)
	}
}

// A structured-condition rule is matched by its conditions, not its Match text.
func TestStructuredConditionRuleCanExplainARow(t *testing.T) {
	cond := rules.Rule{
		ID: "r-cond", SetCategoryID: "c-dining",
		Conditions: []rules.RuleCondition{{Field: rules.ConditionFieldDescription, Op: rules.ConditionOpContains, Value: "coffee"}},
	}
	got := txnprov.Of(txn(nil), []rules.Rule{cond})
	if got.RuleMatch == nil || got.RuleMatch.ID != "r-cond" {
		t.Fatalf("RuleMatch = %v, want the condition rule", got.RuleMatch)
	}
}
