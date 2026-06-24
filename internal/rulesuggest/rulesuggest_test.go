// SPDX-License-Identifier: MIT

package rulesuggest

import (
	"testing"

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
