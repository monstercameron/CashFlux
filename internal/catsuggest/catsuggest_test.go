// SPDX-License-Identifier: MIT

package catsuggest

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
)

// cats mirrors the shape of a real household: a mix of top-level and child
// categories, plus income, so kind-gating is exercised.
func cats() []domain.Category {
	return []domain.Category{
		{ID: "c-groceries", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "c-dining", Name: "Dining", Kind: domain.KindExpense},
		{ID: "c-transport", Name: "Transportation", Kind: domain.KindExpense},
		{ID: "c-gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "c-transport"},
		{ID: "c-shopping", Name: "Shopping", Kind: domain.KindExpense},
		{ID: "c-subs", Name: "Subscriptions", Kind: domain.KindExpense},
		{ID: "c-salary", Name: "Salary", Kind: domain.KindIncome},
	}
}

func TestRuleAlwaysWins(t *testing.T) {
	tally := learntally.Tally{}
	for i := 0; i < 5; i++ {
		tally.Record("STARBUCKS 123", "c-dining")
	}
	got, ok := Resolve(Input{
		Payee:          "STARBUCKS 123",
		AmountMinor:    -650,
		RuleCategoryID: "c-groceries", // deliberately "wrong" — the user said so
		Tally:          tally,
		Categories:     cats(),
	})
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got.CategoryID != "c-groceries" {
		t.Errorf("CategoryID = %q, want c-groceries — a user rule outranks history", got.CategoryID)
	}
	if got.Source != SourceRule || got.Confidence != ConfHigh {
		t.Errorf("Source/Confidence = %v/%v, want rule/high", got.Source, got.Confidence)
	}
}

func TestRuleWithUnknownCategoryFallsThrough(t *testing.T) {
	// A rule pointing at a deleted category must not produce a dangling id.
	got, ok := Resolve(Input{
		Payee:          "STARBUCKS",
		AmountMinor:    -650,
		RuleCategoryID: "c-deleted",
		Categories:     cats(),
	})
	if !ok {
		t.Fatal("expected the dictionary to still resolve it")
	}
	if got.Source == SourceRule {
		t.Error("a rule pointing at a missing category must not be used")
	}
	if got.CategoryID != "c-dining" {
		t.Errorf("CategoryID = %q, want c-dining via the dictionary", got.CategoryID)
	}
}

func TestHistoryBeatsDictionary(t *testing.T) {
	// The dictionary says Starbucks is coffee → Dining. The user has always
	// filed it as Groceries. The user's own behaviour must win.
	tally := learntally.Tally{}
	for i := 0; i < 4; i++ {
		tally.Record("STARBUCKS STORE 123", "c-groceries")
	}
	got, ok := Resolve(Input{
		Payee:       "STARBUCKS STORE 123",
		AmountMinor: -650,
		Tally:       tally,
		Categories:  cats(),
	})
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got.CategoryID != "c-groceries" {
		t.Errorf("CategoryID = %q, want c-groceries — history outranks the dictionary", got.CategoryID)
	}
	if got.Source != SourceHistoryExact {
		t.Errorf("Source = %v, want history-exact", got.Source)
	}
	if got.Count != 4 || got.Total != 4 {
		t.Errorf("evidence = %d/%d, want 4/4", got.Count, got.Total)
	}
}

func TestMerchantHistoryOutranksDictionaryButScoresLower(t *testing.T) {
	tally := learntally.Tally{}
	tally.Record("AMZN MKTP US*111", "c-shopping")
	tally.Record("AMZN MKTP US*222", "c-shopping")
	tally.Record("AMZN MKTP US*333", "c-shopping")

	got, ok := Resolve(Input{
		Payee:       "AMZN MKTP US*NEW",
		AmountMinor: -2314,
		Tally:       tally,
		Categories:  cats(),
	})
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got.Source != SourceHistoryMerchant {
		t.Errorf("Source = %v, want history-merchant", got.Source)
	}
	if got.Confidence != ConfMedium {
		t.Errorf("Confidence = %v, want medium for a consistent merchant history", got.Confidence)
	}
}

func TestSplitHistoryIsLowConfidence(t *testing.T) {
	tally := learntally.Tally{}
	for i := 0; i < 3; i++ {
		tally.Record("TARGET 1234", "c-groceries")
		tally.Record("TARGET 1234", "c-shopping")
	}
	got, ok := Resolve(Input{
		Payee:       "TARGET 1234",
		AmountMinor: -4000,
		Tally:       tally,
		Categories:  cats(),
	})
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got.Confidence != ConfLow {
		t.Errorf("Confidence = %v, want low — a 3/6 split is not an answer", got.Confidence)
	}
}

func TestDictionaryResolvesOnDayOne(t *testing.T) {
	// No rules, no history — the cold start C514 exists for.
	tests := []struct {
		raw  string
		want string
	}{
		{"TRADER JOE'S #453 QPS", "c-groceries"},
		{"SHELL OIL 57445208", "c-gas"},
		{"NETFLIX.COM 866-579-7172", "c-subs"},
		{"AMZN MKTP US*2H4RT9", "c-shopping"},
		{"SQ *BLUE BOTTLE COFFE", "c-dining"}, // coffee falls back to Dining
	}
	for _, tc := range tests {
		got, ok := Resolve(Input{Payee: tc.raw, AmountMinor: -1000, Categories: cats()})
		if !ok {
			t.Errorf("Resolve(%q) found nothing, want %s", tc.raw, tc.want)
			continue
		}
		if got.CategoryID != tc.want {
			t.Errorf("Resolve(%q) = %q, want %q", tc.raw, got.CategoryID, tc.want)
		}
		if got.Source != SourceDictionary {
			t.Errorf("Resolve(%q) source = %v, want dictionary", tc.raw, got.Source)
		}
		if got.Merchant == "" {
			t.Errorf("Resolve(%q) carried no merchant name as evidence", tc.raw)
		}
	}
}

// TestDictionaryNeverInventsACategory: a household without a plausible category
// must get no suggestion rather than a wrong one.
func TestDictionaryNeverInventsACategory(t *testing.T) {
	sparse := []domain.Category{{ID: "c-only", Name: "Everything else", Kind: domain.KindExpense}}
	if got, ok := Resolve(Input{Payee: "SHELL OIL 123", AmountMinor: -5000, Categories: sparse}); ok {
		t.Errorf("resolved to %q against a household with no matching category; want no suggestion", got.CategoryID)
	}
	if got, ok := Resolve(Input{Payee: "SHELL OIL 123", AmountMinor: -5000}); ok {
		t.Errorf("resolved to %q with no categories at all; want no suggestion", got.CategoryID)
	}
}

// TestKindGate: an income transaction must never receive an expense category.
func TestKindGate(t *testing.T) {
	// A positive amount with a merchant the dictionary maps to an EXPENSE bucket.
	if got, ok := Resolve(Input{Payee: "SHELL OIL 123", AmountMinor: 5000, Categories: cats()}); ok {
		t.Errorf("income charge resolved to %q (an expense category); want no suggestion", got.CategoryID)
	}
	// The same merchant as an expense still resolves.
	if _, ok := Resolve(Input{Payee: "SHELL OIL 123", AmountMinor: -5000, Categories: cats()}); !ok {
		t.Error("expense charge should still resolve")
	}
}

func TestUnresolvableIsTheSmartPlusQueue(t *testing.T) {
	for _, raw := range []string{"ACH DEBIT ELAN FIN SVC", "POS DEBIT 4471", "ZZQX HOLDINGS"} {
		if got, ok := Resolve(Input{Payee: raw, AmountMinor: -1000, Categories: cats()}); ok {
			t.Errorf("Resolve(%q) = %q; an unknown merchant must fall through to SMART+", raw, got.CategoryID)
		}
	}
}

func TestDescFallbackWhenPayeeEmpty(t *testing.T) {
	got, ok := Resolve(Input{Desc: "NETFLIX.COM", AmountMinor: -2299, Categories: cats()})
	if !ok {
		t.Fatal("expected the description to be used when payee is empty")
	}
	if got.CategoryID != "c-subs" {
		t.Errorf("CategoryID = %q, want c-subs", got.CategoryID)
	}
}

func TestEmptyInput(t *testing.T) {
	if _, ok := Resolve(Input{Categories: cats()}); ok {
		t.Error("a charge with no payee or description must not resolve")
	}
}

// TestSourceOrderingIsComparable: callers rank with < and >, so the constants
// must stay ordered weakest-to-strongest.
func TestSourceOrderingIsComparable(t *testing.T) {
	if !(SourceNone < SourceDictionary &&
		SourceDictionary < SourceHistoryMerchant &&
		SourceHistoryMerchant < SourceHistoryExact &&
		SourceHistoryExact < SourceRule) {
		t.Error("Source constants are not ordered weakest to strongest")
	}
	if !(ConfNone < ConfLow && ConfLow < ConfMedium && ConfMedium < ConfHigh) {
		t.Error("Confidence constants are not ordered")
	}
}

func TestSourceString(t *testing.T) {
	tests := map[Source]string{
		SourceNone: "none", SourceDictionary: "dictionary",
		SourceHistoryMerchant: "history-merchant", SourceHistoryExact: "history-exact",
		SourceRule: "rule",
	}
	for s, want := range tests {
		if got := s.String(); got != want {
			t.Errorf("Source(%d).String() = %q, want %q", s, got, want)
		}
	}
}
