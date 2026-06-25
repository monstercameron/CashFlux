package learntally

import "testing"

// ---- NormalizePayee --------------------------------------------------------

func TestNormalizePayee(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"already clean", "starbucks", "starbucks"},
		{"uppercase", "Starbucks", "starbucks"},
		{"mixed case", "StArBuCkS", "starbucks"},
		{"leading space", "  starbucks", "starbucks"},
		{"trailing space", "starbucks  ", "starbucks"},
		{"internal spaces collapsed", "starbucks   coffee", "starbucks coffee"},
		{"tab and newline", "whole\tfoods\nmarket", "whole foods market"},
		{"all whitespace", "   ", ""},
		{"unicode letters", "Café Nero", "café nero"},
		{"multiple internal runs", "  Target  \t  Store  ", "target store"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizePayee(tc.input)
			if got != tc.want {
				t.Errorf("NormalizePayee(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---- Increment -------------------------------------------------------------

func TestIncrement_AccumulatesAndNormalizes(t *testing.T) {
	tally := make(Tally)

	// Mixed-case and extra whitespace variants should all map to the same key.
	tally.Increment("Amazon", "shopping")
	tally.Increment("AMAZON", "shopping")
	tally.Increment("  amazon  ", "shopping")
	tally.Increment("amazon", "books")

	cat, n := tally.TopCategory("amazon")
	if cat != "shopping" || n != 3 {
		t.Errorf("TopCategory(amazon) = (%q, %d); want (shopping, 3)", cat, n)
	}

	cat, n = tally.TopCategory("AMAZON")
	if cat != "shopping" || n != 3 {
		t.Errorf("TopCategory(AMAZON) = (%q, %d); want (shopping, 3)", cat, n)
	}
}

func TestIncrement_NoOpOnEmptyPayee(t *testing.T) {
	tally := make(Tally)
	tally.Increment("", "shopping")
	tally.Increment("   ", "shopping")
	if len(tally) != 0 {
		t.Errorf("expected empty tally after no-op increments, got %v", tally)
	}
}

func TestIncrement_NoOpOnEmptyCategoryID(t *testing.T) {
	tally := make(Tally)
	tally.Increment("amazon", "")
	if len(tally) != 0 {
		t.Errorf("expected empty tally after no-op increment with empty categoryID, got %v", tally)
	}
}

func TestIncrement_MultipleCategoriesSeparatelyTracked(t *testing.T) {
	tally := make(Tally)
	tally.Increment("netflix", "entertainment")
	tally.Increment("netflix", "entertainment")
	tally.Increment("netflix", "subscriptions")

	inner := tally["netflix"]
	if inner["entertainment"] != 2 {
		t.Errorf("entertainment count = %d; want 2", inner["entertainment"])
	}
	if inner["subscriptions"] != 1 {
		t.Errorf("subscriptions count = %d; want 1", inner["subscriptions"])
	}
}

// ---- TopCategory -----------------------------------------------------------

func TestTopCategory_ReturnsZeroForUnknownPayee(t *testing.T) {
	tally := make(Tally)
	cat, n := tally.TopCategory("unknown")
	if cat != "" || n != 0 {
		t.Errorf("TopCategory(unknown) = (%q, %d); want (\"\", 0)", cat, n)
	}
}

func TestTopCategory_PicksHighestCount(t *testing.T) {
	tally := make(Tally)
	tally.Increment("walmart", "groceries")
	tally.Increment("walmart", "groceries")
	tally.Increment("walmart", "household")

	cat, n := tally.TopCategory("walmart")
	if cat != "groceries" || n != 2 {
		t.Errorf("TopCategory(walmart) = (%q, %d); want (groceries, 2)", cat, n)
	}
}

func TestTopCategory_TieBreakLexicographicallySmallest(t *testing.T) {
	tally := make(Tally)
	tally.Increment("target", "groceries")
	tally.Increment("target", "household")
	// groceries and household are tied at 1 each; "groceries" < "household".
	cat, n := tally.TopCategory("target")
	if cat != "groceries" || n != 1 {
		t.Errorf("TopCategory(target) = (%q, %d); want (groceries, 1)", cat, n)
	}
}

func TestTopCategory_TieBreakWithMoreCategories(t *testing.T) {
	tally := make(Tally)
	// All three tied at 2; lexicographically: "aaa" < "bbb" < "zzz"
	tally.Increment("costco", "zzz")
	tally.Increment("costco", "zzz")
	tally.Increment("costco", "bbb")
	tally.Increment("costco", "bbb")
	tally.Increment("costco", "aaa")
	tally.Increment("costco", "aaa")

	cat, n := tally.TopCategory("costco")
	if cat != "aaa" || n != 2 {
		t.Errorf("TopCategory(costco) = (%q, %d); want (aaa, 2)", cat, n)
	}
}

// ---- ShouldSuggest ---------------------------------------------------------

func TestShouldSuggest_ReturnsFalseWhenNoData(t *testing.T) {
	tally := make(Tally)
	cat, ok := tally.ShouldSuggest("unknown", DefaultMinCount)
	if ok || cat != "" {
		t.Errorf("ShouldSuggest(unknown) = (%q, %v); want (\"\", false)", cat, ok)
	}
}

func TestShouldSuggest_BelowThreshold(t *testing.T) {
	tally := make(Tally)
	tally.Increment("spotify", "entertainment")
	tally.Increment("spotify", "entertainment")
	// count=2, threshold=DefaultMinCount(3) → no suggestion
	cat, ok := tally.ShouldSuggest("spotify", DefaultMinCount)
	if ok || cat != "" {
		t.Errorf("ShouldSuggest(spotify, 3) = (%q, %v); want (\"\", false)", cat, ok)
	}
}

func TestShouldSuggest_AtThreshold(t *testing.T) {
	tally := make(Tally)
	for range DefaultMinCount {
		tally.Increment("spotify", "entertainment")
	}
	cat, ok := tally.ShouldSuggest("spotify", DefaultMinCount)
	if !ok || cat != "entertainment" {
		t.Errorf("ShouldSuggest(spotify, 3) = (%q, %v); want (entertainment, true)", cat, ok)
	}
}

func TestShouldSuggest_AboveThreshold(t *testing.T) {
	tally := make(Tally)
	for range 10 {
		tally.Increment("hulu", "streaming")
	}
	cat, ok := tally.ShouldSuggest("hulu", DefaultMinCount)
	if !ok || cat != "streaming" {
		t.Errorf("ShouldSuggest(hulu, 3) = (%q, %v); want (streaming, true)", cat, ok)
	}
}

func TestShouldSuggest_ThresholdZeroTreatedAsOne(t *testing.T) {
	tally := make(Tally)
	tally.Increment("apple", "tech")
	cat, ok := tally.ShouldSuggest("apple", 0)
	if !ok || cat != "tech" {
		t.Errorf("ShouldSuggest(apple, 0) = (%q, %v); want (tech, true)", cat, ok)
	}
}

func TestShouldSuggest_ThresholdNegativeTreatedAsOne(t *testing.T) {
	tally := make(Tally)
	tally.Increment("apple", "tech")
	cat, ok := tally.ShouldSuggest("apple", -5)
	if !ok || cat != "tech" {
		t.Errorf("ShouldSuggest(apple, -5) = (%q, %v); want (tech, true)", cat, ok)
	}
}

// ---- DefaultMinCount -------------------------------------------------------

func TestDefaultMinCount_Value(t *testing.T) {
	if DefaultMinCount != 3 {
		t.Errorf("DefaultMinCount = %d; want 3", DefaultMinCount)
	}
}
