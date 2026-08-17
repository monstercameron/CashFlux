// SPDX-License-Identifier: MIT

package rulesuggest

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func tx(payee, desc, cat string) domain.Transaction {
	return domain.Transaction{Payee: payee, Desc: desc, CategoryID: cat, Amount: money.New(-500, "USD")}
}

func TestSuggest(t *testing.T) {
	txns := []domain.Transaction{
		tx("Starbucks", "coffee", "cafe"),
		tx("Starbucks", "latte", "cafe"),
		tx("Starbucks", "beans", "cafe"), // 3× Starbucks → cafe (consistent) → suggested
		tx("Shell", "fuel", "gas"),
		tx("Shell", "fuel", "gas"), // only 2× but minCount 3 → not suggested
		tx("Costco", "bulk", "food"),
		tx("Costco", "bulk", "food"),
		tx("Costco", "tires", "auto"),                                 // 3× Costco but mixed 2/3 food < 0.8 → not suggested
		tx("", "", "food"),                                            // empty key → ignored
		{Payee: "Transfer", CategoryID: "x", TransferAccountID: "a2"}, // transfer → ignored
	}
	got := Suggest(txns, nil, 3)
	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1: %+v", len(got), got)
	}
	s := got[0]
	if s.Rule.Match != "Starbucks" || s.Rule.SetCategoryID != "cafe" || s.Support != 3 || s.Total != 3 {
		t.Errorf("suggestion = %+v, want Starbucks→cafe (3/3)", s)
	}
}

func TestSuggestSkipsExisting(t *testing.T) {
	txns := []domain.Transaction{
		tx("Starbucks", "coffee", "cafe"),
		tx("Starbucks", "latte", "cafe"),
		tx("Starbucks", "beans", "cafe"),
	}
	existing := []rules.Rule{{Match: "starbucks", SetCategoryID: "cafe"}}
	if got := Suggest(txns, existing, 3); len(got) != 0 {
		t.Errorf("expected no suggestions when a rule already matches, got %+v", got)
	}
}

func TestSuggestSortsBySupport(t *testing.T) {
	var txns []domain.Transaction
	for i := 0; i < 5; i++ {
		txns = append(txns, tx("Amazon", "order", "shopping"))
	}
	for i := 0; i < 3; i++ {
		txns = append(txns, tx("Netflix", "sub", "entertainment"))
	}
	got := Suggest(txns, nil, 3)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2: %+v", len(got), got)
	}
	if got[0].Rule.Match != "Amazon" || got[1].Rule.Match != "Netflix" {
		t.Errorf("order = %q, %q; want Amazon then Netflix (by support)", got[0].Rule.Match, got[1].Rule.Match)
	}
}

func TestSuggestUsesDescWhenNoPayee(t *testing.T) {
	txns := []domain.Transaction{
		tx("", "Monthly gym membership", "health"),
		tx("", "Monthly gym membership", "health"),
		tx("", "Monthly gym membership", "health"),
	}
	got := Suggest(txns, nil, 3)
	if len(got) != 1 || got[0].Rule.Match != "Monthly gym membership" {
		t.Errorf("want a desc-based suggestion, got %+v", got)
	}
}

// TestSuggestSkipsConditionCoveredKeys proves a condition-bearing rule (which
// the legacy substring check can't see) suppresses suggestions for a key whose
// transactions it already governs.
func TestSuggestSkipsConditionCoveredKeys(t *testing.T) {
	txns := []domain.Transaction{
		tx("Starbucks", "coffee", "cafe"),
		tx("Starbucks", "latte", "cafe"),
		tx("Starbucks", "beans", "cafe"),
	}
	// A condition rule catching every small outflow (amount > -1000 minor units,
	// i.e. all the -500 fixtures) already governs the Starbucks population.
	existing := []rules.Rule{{
		ID: "small", SetCategoryID: "cafe",
		Conditions: []rules.RuleCondition{{Field: rules.ConditionFieldAmount, Op: rules.ConditionOpGt, Value: "-1000"}},
	}}
	if got := Suggest(txns, existing, 3); len(got) != 0 {
		t.Fatalf("Suggest = %d suggestions, want 0 (key fully governed by a condition rule)", len(got))
	}
	// Without the rule the key IS suggested (sanity check).
	if got := Suggest(txns, nil, 3); len(got) != 1 {
		t.Fatalf("Suggest without rules = %d, want 1", len(got))
	}
}

// ─── LF-10: asking about one payee ───────────────────────────────────────────

func fpTxn(id, payee, cat string) domain.Transaction {
	return domain.Transaction{ID: id, Payee: payee, CategoryID: cat, Desc: payee,
		Date:   time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-2500, "USD")}
}

// A surface that already knows the merchant must not re-derive what "confident
// enough" means — two definitions is how a merchant gets offered a rule on one
// screen and refused it on another.
func TestForPayeeAgreesWithSuggest(t *testing.T) {
	txns := []domain.Transaction{
		fpTxn("a", "Greenfield Market", "c-groceries"),
		fpTxn("b", "Greenfield Market", "c-groceries"),
		fpTxn("c", "Greenfield Market", "c-groceries"),
		fpTxn("d", "Rare Shop", "c-misc"),
	}
	bulk := Suggest(txns, nil, 3)
	one, ok := ForPayee(txns, nil, "Greenfield Market", 3)
	if !ok {
		t.Fatal("ForPayee refused a payee Suggest proposed")
	}
	var found bool
	for _, s := range bulk {
		if s.Rule.Match == one.Rule.Match && s.Rule.SetCategoryID == one.Rule.SetCategoryID &&
			s.Support == one.Support && s.Total == one.Total {
			found = true
		}
	}
	if !found {
		t.Errorf("ForPayee = %+v, which Suggest did not propose: %+v", one, bulk)
	}
	// A payee below the threshold is refused, exactly as the bulk path skips it.
	if _, ok := ForPayee(txns, nil, "Rare Shop", 3); ok {
		t.Error("ForPayee proposed a rule for a payee Suggest skipped")
	}
}

func TestForPayeeIsCaseAndWhitespaceInsensitive(t *testing.T) {
	txns := []domain.Transaction{
		fpTxn("a", "Greenfield Market", "c-groceries"),
		fpTxn("b", "Greenfield Market", "c-groceries"),
	}
	if _, ok := ForPayee(txns, nil, "  greenfield market ", 2); !ok {
		t.Error("a differently-cased payee was not matched")
	}
}

func TestForPayeeGuards(t *testing.T) {
	txns := []domain.Transaction{fpTxn("a", "X", "c"), fpTxn("b", "X", "c")}
	if _, ok := ForPayee(txns, nil, "", 2); ok {
		t.Error("an empty payee produced a suggestion")
	}
	if _, ok := ForPayee(nil, nil, "X", 2); ok {
		t.Error("no transactions produced a suggestion")
	}
	// An existing rule already covering the payee means nothing to suggest.
	if _, ok := ForPayee(txns, []rules.Rule{{ID: "r", Match: "X", SetCategoryID: "c"}}, "X", 2); ok {
		t.Error("a payee already covered by a rule was proposed again")
	}
}
