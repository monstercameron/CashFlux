// SPDX-License-Identifier: MIT

package catsuggest_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
)

func cats() []domain.Category {
	return []domain.Category{
		{ID: "c-groceries", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "c-dining", Name: "Dining", Kind: domain.KindExpense},
		{ID: "c-shopping", Name: "Shopping", Kind: domain.KindExpense},
		{ID: "c-transport", Name: "Transportation", Kind: domain.KindExpense},
		{ID: "c-gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "c-transport"},
		{ID: "c-salary", Name: "Salary", Kind: domain.KindIncome},
	}
}

// TestPrecedenceMatrix walks EVERY combination of the four sources and asserts
// the winner. The order — rule > exact history > merchant history > dictionary >
// nothing — is the contract Cam set, and it is the one thing in this package a
// future change is most likely to break silently, because every individual
// source keeps working when the ranking is wrong.
func TestPrecedenceMatrix(t *testing.T) {
	// "STARBUCKS ..." is in the shipped dictionary, so the dictionary source can
	// be switched on and off by choosing the base name.
	const knownBase, unknownBase = "STARBUCKS STORE", "ZZQX HOLDINGS LLC"

	tests := []struct {
		name                                   string
		rule, exactHist, merchHist, dictionary bool
		wantSource                             catsuggest.Source
		wantCat                                string
	}{
		{"nothing", false, false, false, false, catsuggest.SourceNone, ""},
		{"dictionary only", false, false, false, true, catsuggest.SourceDictionary, "c-dining"},
		{"merchant history only", false, false, true, false, catsuggest.SourceHistoryMerchant, "c-shopping"},
		{"merchant history beats dictionary", false, false, true, true, catsuggest.SourceHistoryMerchant, "c-shopping"},
		{"exact history only", false, true, false, false, catsuggest.SourceHistoryExact, "c-groceries"},
		{"exact beats merchant", false, true, true, false, catsuggest.SourceHistoryExact, "c-groceries"},
		{"exact beats dictionary", false, true, false, true, catsuggest.SourceHistoryExact, "c-groceries"},
		{"exact beats both", false, true, true, true, catsuggest.SourceHistoryExact, "c-groceries"},
		{"rule only", true, false, false, false, catsuggest.SourceRule, "c-salary"},
		{"rule beats dictionary", true, false, false, true, catsuggest.SourceRule, "c-salary"},
		{"rule beats merchant history", true, false, true, false, catsuggest.SourceRule, "c-salary"},
		{"rule beats exact history", true, true, false, false, catsuggest.SourceRule, "c-salary"},
		{"rule beats everything", true, true, true, true, catsuggest.SourceRule, "c-salary"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := unknownBase
			if tc.dictionary {
				base = knownBase
			}
			// The charge under test carries its own per-charge reference, exactly
			// as a real descriptor does.
			payee := base + " 999"

			tally := learntally.Tally{}
			if tc.merchHist {
				// Three OTHER descriptors from the same merchant. Trailing numeric
				// references are stripped by normalization, so these share a stem
				// key with `payee` while every exact key differs — which is the
				// only way to exercise the merchant tier without the exact one.
				for _, d := range []string{base + " 111", base + " 222", base + " 333"} {
					tally.Record(d, "c-shopping")
				}
			}
			if tc.exactHist {
				for i := 0; i < 3; i++ {
					tally.Record(payee, "c-groceries")
				}
			}
			ruleCat := ""
			if tc.rule {
				// Deliberately a category none of the other sources would pick, and
				// deliberately income on an expense charge: a rule is the user's
				// explicit instruction and outranks even the sign heuristic.
				ruleCat = "c-salary"
			}

			got, ok := catsuggest.Resolve(catsuggest.Input{
				Payee:          payee,
				AmountMinor:    -650,
				RuleCategoryID: ruleCat,
				Tally:          tally,
				Categories:     cats(),
			})
			if tc.wantSource == catsuggest.SourceNone {
				if ok {
					t.Fatalf("expected no suggestion, got %+v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("expected a %v suggestion, got none", tc.wantSource)
			}
			if got.Source != tc.wantSource {
				t.Errorf("Source = %v, want %v", got.Source, tc.wantSource)
			}
			if got.CategoryID != tc.wantCat {
				t.Errorf("CategoryID = %q, want %q", got.CategoryID, tc.wantCat)
			}
		})
	}
}

// TestResolveNeverReturnsACategoryTheHouseholdDoesNotHave is the invariant that
// keeps a suggestion safe to WRITE. A dangling id would be persisted onto a
// transaction and then render as blank everywhere.
func TestResolveNeverReturnsACategoryTheHouseholdDoesNotHave(t *testing.T) {
	all := cats()
	valid := map[string]domain.Category{}
	for _, c := range all {
		valid[c.ID] = c
	}
	payees := []string{
		"STARBUCKS", "TRADER JOE'S #453", "SHELL OIL 1", "NETFLIX.COM",
		"ZZQX HOLDINGS", "", "   ", "\x00", "AMZN MKTP US*1",
	}
	amounts := []int64{-650, 650, 0}
	rules := []string{"", "c-groceries", "c-deleted", "   "}

	for _, p := range payees {
		for _, amt := range amounts {
			for _, r := range rules {
				tally := learntally.Tally{}
				for i := 0; i < 4; i++ {
					tally.Record(p, "c-dining")
				}
				// A tally entry pointing at a category that no longer exists must
				// never leak through either.
				tally.Record(p, "c-deleted")

				got, ok := catsuggest.Resolve(catsuggest.Input{
					Payee: p, AmountMinor: amt, RuleCategoryID: r,
					Tally: tally, Categories: all,
				})
				if !ok {
					continue
				}
				c, exists := valid[got.CategoryID]
				if !exists {
					t.Fatalf("payee=%q amt=%d rule=%q resolved to %q, which does not exist",
						p, amt, r, got.CategoryID)
				}
				// And the kind must match the sign, with one documented exception:
				// an explicit user RULE outranks the heuristic.
				if got.Source != catsuggest.SourceRule {
					wantIncome := amt > 0
					if (c.Kind == domain.KindIncome) != wantIncome {
						t.Errorf("payee=%q amt=%d gave %q (kind %v) — wrong kind for the sign",
							p, amt, c.Name, c.Kind)
					}
				}
				if got.Confidence < catsuggest.ConfLow || got.Confidence > catsuggest.ConfHigh {
					t.Errorf("confidence %v outside the enum", got.Confidence)
				}
			}
		}
	}
}

// TestResolveIsDeterministic: the surface rebuilds its index on every render, so
// a suggestion that wobbled would make rows jump between tiers as you look at them.
func TestResolveIsDeterministic(t *testing.T) {
	tally := learntally.Tally{}
	// A deliberately TIED history: two categories with equal counts is exactly
	// where a map-iteration-order bug would show up.
	for i := 0; i < 4; i++ {
		tally.Record("TARGET 1234", "c-groceries")
		tally.Record("TARGET 1234", "c-shopping")
	}
	in := catsuggest.Input{Payee: "TARGET 1234", AmountMinor: -4000, Tally: tally, Categories: cats()}
	first, ok1 := catsuggest.Resolve(in)
	for i := 0; i < 200; i++ {
		got, ok := catsuggest.Resolve(in)
		if ok != ok1 || got != first {
			t.Fatalf("Resolve is not deterministic: %+v/%v then %+v/%v", first, ok1, got, ok)
		}
	}
}

// TestEmptyCategoryListNeverResolves: a household mid-setup has no categories,
// and inventing one for them is worse than staying quiet.
func TestEmptyCategoryListNeverResolves(t *testing.T) {
	tally := learntally.Tally{}
	for i := 0; i < 5; i++ {
		tally.Record("STARBUCKS", "c-dining")
	}
	for _, in := range []catsuggest.Input{
		{Payee: "STARBUCKS", AmountMinor: -650},
		{Payee: "STARBUCKS", AmountMinor: -650, Tally: tally},
		{Payee: "STARBUCKS", AmountMinor: -650, RuleCategoryID: "c-dining"},
		{Payee: "STARBUCKS", AmountMinor: -650, Categories: []domain.Category{}},
	} {
		if got, ok := catsuggest.Resolve(in); ok {
			t.Errorf("resolved to %q with no categories available", got.CategoryID)
		}
	}
}

// TestEvidenceMatchesTheSource: the UI phrases the evidence from these fields,
// so a count attached to a dictionary hit (or a merchant name attached to a
// rule) would produce a sentence that is simply untrue.
func TestEvidenceMatchesTheSource(t *testing.T) {
	tally := learntally.Tally{}
	for i := 0; i < 4; i++ {
		tally.Record("STARBUCKS STORE 1", "c-dining")
	}
	cases := []catsuggest.Input{
		{Payee: "STARBUCKS", AmountMinor: -650, Categories: cats()},                            // dictionary
		{Payee: "STARBUCKS STORE 1", AmountMinor: -650, Tally: tally, Categories: cats()},      // history
		{Payee: "ANYTHING", AmountMinor: -650, RuleCategoryID: "c-dining", Categories: cats()}, // rule
	}
	for _, in := range cases {
		got, ok := catsuggest.Resolve(in)
		if !ok {
			t.Fatalf("expected a suggestion for %+v", in.Payee)
		}
		switch got.Source {
		case catsuggest.SourceRule:
			if got.Count != 0 || got.Total != 0 || got.Merchant != "" {
				t.Errorf("a rule carried evidence it cannot have: %+v", got)
			}
		case catsuggest.SourceDictionary:
			if got.Merchant == "" {
				t.Errorf("a dictionary hit carried no merchant name: %+v", got)
			}
			if got.Count != 0 || got.Total != 0 {
				t.Errorf("a dictionary hit carried history counts: %+v", got)
			}
		case catsuggest.SourceHistoryExact, catsuggest.SourceHistoryMerchant:
			if got.Count <= 0 || got.Total < got.Count {
				t.Errorf("history evidence is incoherent: %d of %d", got.Count, got.Total)
			}
			if got.Merchant != "" {
				t.Errorf("a history hit carried a dictionary merchant name: %+v", got)
			}
		}
	}
}

// TestMoreConsistentHistoryIsNeverLessConfident: adding agreeing evidence must
// not lower confidence, or the tiering would punish users for being consistent.
func TestMoreConsistentHistoryIsNeverLessConfident(t *testing.T) {
	prev := catsuggest.ConfNone
	for n := 3; n <= 12; n++ {
		tally := learntally.Tally{}
		for i := 0; i < n; i++ {
			tally.Record("SOMEPLACE 1", "c-dining")
		}
		got, ok := catsuggest.Resolve(catsuggest.Input{
			Payee: "SOMEPLACE 1", AmountMinor: -100, Tally: tally, Categories: cats(),
		})
		if !ok {
			t.Fatalf("n=%d produced no suggestion", n)
		}
		if got.Confidence < prev {
			t.Errorf("confidence FELL from %v to %v as evidence grew to %d", prev, got.Confidence, n)
		}
		prev = got.Confidence
	}
}
