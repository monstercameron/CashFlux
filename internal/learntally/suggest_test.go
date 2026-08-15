// SPDX-License-Identifier: MIT

package learntally

import (
	"strings"
	"testing"
)

// TestMerchantKeyCollapsesPerChargeReferences is the whole point of C513: the
// exact-descriptor key can never accumulate for a merchant that stamps a
// reference on every charge.
func TestMerchantKeyCollapsesPerChargeReferences(t *testing.T) {
	descriptors := []string{
		"AMZN MKTP US*2H4RT9",
		"AMZN MKTP US*8K1QP2",
		"AMZN MKTP US*4J0RT1",
	}
	first := MerchantKey(descriptors[0])
	if first == "" {
		t.Fatal("MerchantKey returned empty for a real descriptor")
	}
	for _, d := range descriptors[1:] {
		if got := MerchantKey(d); got != first {
			t.Errorf("MerchantKey(%q) = %q, want %q — per-charge references must collapse", d, got, first)
		}
	}
	// And the exact keys must still differ, or the two tiers would be identical.
	if NormalizePayee(descriptors[0]) == NormalizePayee(descriptors[1]) {
		t.Error("exact keys collapsed too; the exact tier would be meaningless")
	}
}

func TestMerchantKeyEmptyInput(t *testing.T) {
	for _, in := range []string{"", "   ", "\t"} {
		if got := MerchantKey(in); got != "" {
			t.Errorf("MerchantKey(%q) = %q, want empty", in, got)
		}
	}
}

// TestSuggestFromMerchantHistory: varying descriptors that would never clear the
// threshold individually now do so together.
func TestSuggestFromMerchantHistory(t *testing.T) {
	tally := Tally{}
	tally.Record("AMZN MKTP US*2H4RT9", "cat-shopping")
	tally.Record("AMZN MKTP US*8K1QP2", "cat-shopping")
	tally.Record("AMZN MKTP US*4J0RT1", "cat-shopping")

	// A brand-new Amazon descriptor the user has never seen before.
	got, ok := tally.Suggest("AMZN MKTP US*9Z9Z9Z", DefaultMinCount)
	if !ok {
		t.Fatal("no suggestion; three same-merchant corrections should clear the threshold")
	}
	if got.CategoryID != "cat-shopping" {
		t.Errorf("CategoryID = %q, want cat-shopping", got.CategoryID)
	}
	if got.Match != MatchMerchant {
		t.Errorf("Match = %v, want MatchMerchant", got.Match)
	}
	if got.Count != 3 || got.Total != 3 {
		t.Errorf("Count/Total = %d/%d, want 3/3", got.Count, got.Total)
	}
	if !got.Consistent() {
		t.Error("3/3 history should be Consistent")
	}
}

// TestSuggestPrefersTheExactDescriptor: an exact repeat is stronger evidence
// than a merchant-level pattern and must win.
func TestSuggestPrefersTheExactDescriptor(t *testing.T) {
	tally := Tally{}
	// Merchant-level history says Shopping...
	tally.Record("AMZN MKTP US*111", "cat-shopping")
	tally.Record("AMZN MKTP US*222", "cat-shopping")
	tally.Record("AMZN MKTP US*333", "cat-shopping")
	// ...but this precise descriptor has always been Household.
	for i := 0; i < 3; i++ {
		tally.Record("AMZN MKTP US*SUBSCRIBE", "cat-household")
	}

	got, ok := tally.Suggest("AMZN MKTP US*SUBSCRIBE", DefaultMinCount)
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got.CategoryID != "cat-household" {
		t.Errorf("CategoryID = %q, want cat-household (exact beats merchant)", got.CategoryID)
	}
	if got.Match != MatchExact {
		t.Errorf("Match = %v, want MatchExact", got.Match)
	}
}

func TestSuggestBelowThreshold(t *testing.T) {
	tally := Tally{}
	tally.Record("SHELL OIL 123", "cat-gas")
	tally.Record("SHELL OIL 456", "cat-gas")
	if _, ok := tally.Suggest("SHELL OIL 789", 3); ok {
		t.Error("two corrections must not clear a threshold of three")
	}
	tally.Record("SHELL OIL 789", "cat-gas")
	if _, ok := tally.Suggest("SHELL OIL 999", 3); !ok {
		t.Error("three corrections should clear a threshold of three")
	}
}

// TestSuggestReportsSplitHistoryHonestly: the package must not hide a coin flip.
func TestSuggestReportsSplitHistoryHonestly(t *testing.T) {
	tally := Tally{}
	for i := 0; i < 4; i++ {
		tally.Record("TARGET T-1234", "cat-groceries")
	}
	for i := 0; i < 3; i++ {
		tally.Record("TARGET T-1234", "cat-household")
	}
	got, ok := tally.Suggest("TARGET T-1234", 3)
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got.Count != 4 || got.Total != 7 {
		t.Errorf("Count/Total = %d/%d, want 4/7", got.Count, got.Total)
	}
	if !got.Consistent() {
		t.Error("4 of 7 is a strict majority, so Consistent should be true")
	}

	// Now make it a genuine tie: 4/4/... no majority.
	tie := Tally{}
	for i := 0; i < 4; i++ {
		tie.Record("SOMEWHERE", "cat-a")
		tie.Record("SOMEWHERE", "cat-b")
	}
	tied, ok := tie.Suggest("SOMEWHERE", 3)
	if !ok {
		t.Fatal("expected a suggestion even for a tie")
	}
	if tied.Consistent() {
		t.Errorf("4 of 8 is not a majority; Consistent should be false (got %d/%d)", tied.Count, tied.Total)
	}
	if tied.CategoryID != "cat-a" {
		t.Errorf("tie should break deterministically to cat-a, got %q", tied.CategoryID)
	}
}

func TestSuggestNoHistory(t *testing.T) {
	tally := Tally{}
	if _, ok := tally.Suggest("NEVER SEEN BEFORE", 3); ok {
		t.Error("an empty tally must not suggest anything")
	}
	if _, ok := tally.Suggest("", 3); ok {
		t.Error("an empty payee must not suggest anything")
	}
}

func TestSuggestDefaultsThreshold(t *testing.T) {
	tally := Tally{}
	tally.Record("NETFLIX.COM", "cat-subs")
	tally.Record("NETFLIX.COM", "cat-subs")
	if _, ok := tally.Suggest("NETFLIX.COM", 0); ok {
		t.Error("threshold <= 0 should fall back to DefaultMinCount (3), not 1")
	}
	tally.Record("NETFLIX.COM", "cat-subs")
	if _, ok := tally.Suggest("NETFLIX.COM", 0); !ok {
		t.Error("three corrections should clear the defaulted threshold")
	}
}

// TestRecordKeepsIncrementCompatible: Increment is still used by quick-add, so
// Record must not change what that caller sees under the exact key.
func TestRecordKeepsIncrementCompatible(t *testing.T) {
	a, b := Tally{}, Tally{}
	a.Increment("STARBUCKS 123", "cat-coffee")
	b.Record("STARBUCKS 123", "cat-coffee")

	exact := NormalizePayee("STARBUCKS 123")
	if a[exact]["cat-coffee"] != b[exact]["cat-coffee"] {
		t.Error("Record must write the same exact-key entry Increment does")
	}
	if _, ok := a[MerchantKey("STARBUCKS 123")]; ok {
		t.Error("Increment must NOT write a merchant key; only Record does")
	}
	if _, ok := b[MerchantKey("STARBUCKS 123")]; !ok {
		t.Error("Record must write a merchant key")
	}
}

func TestRecordIgnoresEmpty(t *testing.T) {
	tally := Tally{}
	tally.Record("", "cat-x")
	tally.Record("SOMEWHERE", "")
	tally.Record("   ", "cat-x")
	if len(tally) != 0 {
		t.Errorf("empty inputs wrote %d keys, want 0", len(tally))
	}
}

// TestMerchantKeysAreNamespaced: the two tiers share one persisted map, so their
// keys must not be able to collide.
func TestMerchantKeysAreNamespaced(t *testing.T) {
	tally := Tally{}
	tally.Record("Amazon", "cat-shopping")
	for k := range tally {
		if k == "" {
			t.Fatal("empty key written")
		}
	}
	mk := MerchantKey("Amazon")
	if mk == NormalizePayee("Amazon") {
		t.Error("merchant and exact keys must differ even when the payee is already clean")
	}
	// The namespace must be UNFORGEABLE, not merely distinct: an exact key can
	// never begin with a space because NormalizePayee trims.
	if !strings.HasPrefix(mk, " ") {
		t.Errorf("merchant key %q is not namespaced with an unreachable prefix", mk)
	}
	if NormalizePayee("~amazon") == mk {
		t.Error("a payee could forge the merchant namespace")
	}
}
