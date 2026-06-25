// SPDX-License-Identifier: MIT

package help

import (
	"strings"
	"testing"
)

// TestItemsNonEmpty verifies that Items returns at least one entry and that
// every entry has a non-empty Question and Answer.
func TestItemsNonEmpty(t *testing.T) {
	items := Items()
	if len(items) == 0 {
		t.Fatal("Items() returned an empty slice")
	}
	for i, item := range items {
		if strings.TrimSpace(item.Question) == "" {
			t.Errorf("items[%d].Question is empty", i)
		}
		if strings.TrimSpace(item.Answer) == "" {
			t.Errorf("items[%d].Answer is empty", i)
		}
	}
}

// TestItemsIsolation confirms that mutating the returned slice does not affect
// a subsequent call (Items() must return an independent copy).
func TestItemsIsolation(t *testing.T) {
	a := Items()
	a[0].Question = "MUTATED"
	b := Items()
	if b[0].Question == "MUTATED" {
		t.Error("Items() shares backing storage — mutation of one call's slice affected the next")
	}
}

func TestFilter(t *testing.T) {
	all := Items()

	tests := []struct {
		name      string
		query     string
		wantEmpty bool
		// checkFn is called with the result when non-nil; optional.
		checkFn func(t *testing.T, got []FAQItem)
	}{
		{
			name:      "empty query returns all items",
			query:     "",
			wantEmpty: false,
			checkFn: func(t *testing.T, got []FAQItem) {
				if len(got) != len(all) {
					t.Errorf("empty query: want %d items, got %d", len(all), len(got))
				}
			},
		},
		{
			name:      "whitespace-only query returns all items",
			query:     "   ",
			wantEmpty: false,
			checkFn: func(t *testing.T, got []FAQItem) {
				if len(got) != len(all) {
					t.Errorf("whitespace query: want %d items, got %d", len(all), len(got))
				}
			},
		},
		{
			name:  "filter by question word (data)",
			query: "data",
			checkFn: func(t *testing.T, got []FAQItem) {
				if len(got) == 0 {
					t.Error("query 'data' matched nothing; expected at least one item")
				}
				for _, item := range got {
					combined := strings.ToLower(item.Question + item.Answer + strings.Join(item.Keywords, " "))
					if !strings.Contains(combined, "data") {
						t.Errorf("item %q returned but does not contain 'data'", item.Question)
					}
				}
			},
		},
		{
			name:  "filter by keyword (csv)",
			query: "csv",
			checkFn: func(t *testing.T, got []FAQItem) {
				if len(got) == 0 {
					t.Error("query 'csv' matched nothing; expected at least one item")
				}
			},
		},
		{
			name:  "filter is case-insensitive (CSV upper-case)",
			query: "CSV",
			checkFn: func(t *testing.T, got []FAQItem) {
				lower := Filter(all, "csv")
				if len(got) != len(lower) {
					t.Errorf("case sensitivity: 'CSV' returned %d items, 'csv' returned %d", len(got), len(lower))
				}
			},
		},
		{
			name:  "filter matches answer text",
			query: "github",
			checkFn: func(t *testing.T, got []FAQItem) {
				if len(got) == 0 {
					t.Error("query 'github' matched nothing; expected the bug-report FAQ entry")
				}
			},
		},
		{
			name:      "no-match query returns empty slice",
			query:     "xyzzy_no_match_expected",
			wantEmpty: true,
		},
		{
			name:  "filter by keyboard shortcut keyword",
			query: "shortcut",
			checkFn: func(t *testing.T, got []FAQItem) {
				if len(got) == 0 {
					t.Error("query 'shortcut' matched nothing; expected the keyboard shortcuts FAQ entry")
				}
			},
		},
		{
			name:  "filter by partial answer word (encrypted)",
			query: "encrypt",
			checkFn: func(t *testing.T, got []FAQItem) {
				if len(got) == 0 {
					t.Error("query 'encrypt' matched nothing; expected at least one entry to mention encryption")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Filter(all, tc.query)
			if tc.wantEmpty && len(got) != 0 {
				t.Errorf("query %q: want empty result, got %d items", tc.query, len(got))
				return
			}
			if !tc.wantEmpty && tc.checkFn == nil && len(got) == 0 {
				t.Errorf("query %q: want non-empty result, got 0 items", tc.query)
			}
			if tc.checkFn != nil {
				tc.checkFn(t, got)
			}
		})
	}
}

// TestFilterDoesNotMutateInput confirms Filter is a pure function that does
// not modify the items slice passed to it.
func TestFilterDoesNotMutateInput(t *testing.T) {
	items := Items()
	orig := make([]FAQItem, len(items))
	copy(orig, items)

	Filter(items, "csv")

	for i, item := range items {
		if item.Question != orig[i].Question || item.Answer != orig[i].Answer {
			t.Errorf("Filter mutated items[%d]", i)
		}
	}
}
