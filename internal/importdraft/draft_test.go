// SPDX-License-Identifier: MIT

package importdraft

import (
	"math"
	"slices"
	"testing"
	"testing/quick"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/staging"
)

func TestAppendPreservesEditedRowsAndDistinctOccurrences(t *testing.T) {
	m := New([]extract.Row{{ID: "draft-1", Description: "first"}, {ID: "draft-1", Description: "second"}})
	first := m.Rows[0]
	first.Description = "edited"
	m.Replace(first.ID, first)
	m.Append("pdf", []extract.Row{first, {Description: "third\n\u202e shop"}})
	if len(m.Rows) != 4 || m.Rows[0].Description != "edited" || m.Rows[3].Description != "third  shop" {
		t.Fatal(m.Rows)
	}
	seen := map[string]bool{}
	for _, r := range m.Rows {
		if r.ID == "" || seen[r.ID] {
			t.Fatal("reused identity", m.Rows)
		}
		seen[r.ID] = true
	}
	if m.Stage != "review" || m.Source != "pdf" {
		t.Fatal(m)
	}
}

func TestStaleRepliesIgnoredForEveryMutation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Model)
	}{
		{"edit", func(m *Model) { m.Replace(m.Rows[0].ID, m.Rows[0]) }},
		{"remove", func(m *Model) { m.Remove(m.Rows[0].ID) }},
		{"append", func(m *Model) { m.Append("image", []extract.Row{{Description: "new"}}) }},
		{"reset", func(m *Model) { m.Reset() }},
		{"cancel", func(m *Model) { m.Cancel() }},
		{"close", func(m *Model) { m.Close() }},
		{"new request", func(m *Model) { m.Begin() }},
		{"outcomes", func(m *Model) { m.RetainFailures(nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New([]extract.Row{{Description: "first"}})
			request := m.Begin()
			tt.change(m)
			if m.Accept(request) {
				t.Fatal("stale callback accepted")
			}
		})
	}
	m := New(nil)
	if m.Accept(Request{}) {
		t.Fatal("empty request accepted")
	}
	r := m.Begin()
	if !m.Accept(r) || m.Accept(r) {
		t.Fatal("reply must be accepted exactly once")
	}
}

func TestIdentityEditsCategoriesAndFailedRows(t *testing.T) {
	m := New([]extract.Row{{ID: "a", Category: "user"}, {ID: "b"}, {ID: "c"}, {ID: "d"}})
	before := m.Rows
	if m.Replace("missing", extract.Row{}) || m.Remove("missing") {
		t.Fatal("missing identity mutated draft")
	}
	m.ApplyCategories(map[string]string{"a": "AI", "b": "Food", "missing": "Travel"})
	if m.Rows[0].Category != "user" || m.Rows[1].Category != "Food" || &before[0] == &m.Rows[0] {
		t.Fatal(m.Rows)
	}
	m.RetainFailures([]Outcome{{"a", "imported", ""}, {"b", "duplicate", ""}, {"c", "failed", "bad date"}})
	if len(m.Rows) != 2 || m.Rows[0].ID != "c" || m.Errors["c"] != "bad date" {
		t.Fatal(m)
	}
	m.Replace("c", extract.Row{Description: "fixed"})
	if m.Errors["c"] != "" || m.Rows[0].ID != "c" {
		t.Fatal(m)
	}
	m.Remove("c")
	m.Remove("d")
	if m.Stage != "input" {
		t.Fatal(m)
	}
	m.Append("", nil)
	if len(m.Rows) != 0 {
		t.Fatal(m)
	}
}

func TestRemovalNeverChangesOtherIdentitiesProperty(t *testing.T) {
	property := func(count uint8, remove uint8) bool {
		n := int(count)%64 + 1
		pos := int(remove) % n
		m := New(make([]extract.Row, n))
		original := append([]extract.Row(nil), m.Rows...)
		m.Remove(original[pos].ID)
		want := append(original[:pos:pos], original[pos+1:]...)
		return slices.Equal(m.Rows, want)
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatal(err)
	}
}

func TestWindowAndFooterTables(t *testing.T) {
	for _, tt := range []struct{ total, page, size, start, end, current, pages int }{
		{0, 0, 100, 0, 0, 1, 1}, {10, 2, 0, 0, 0, 1, 1}, {201, 0, 100, 0, 100, 1, 3},
		{201, 99, 100, 200, 201, 3, 3}, {201, 2, 100, 100, 200, 2, 3},
	} {
		a, b, c, d := Window(tt.total, tt.page, tt.size)
		if a != tt.start || b != tt.end || c != tt.current || d != tt.pages {
			t.Fatal(tt, a, b, c, d)
		}
	}
	for _, tt := range []struct {
		stage, source string
		rows          int
		preview, busy bool
		want          string
	}{
		{"review", "stmt", 3, false, true, "wait"}, {"review", "stmt", 3, false, false, "review"},
		{"input", "csv", 0, false, false, "csv-preview"}, {"input", "csv", 0, true, false, "csv-confirm"},
		{"input", "pdf", 3, false, false, "show-review"}, {"input", "stmt", 0, false, false, "done"},
	} {
		if got := FooterAction(tt.stage, tt.source, tt.rows, tt.preview, tt.busy); got != tt.want {
			t.Fatal(tt, got)
		}
	}
}

func TestSummaryCheckedTotalsAndVerdicts(t *testing.T) {
	b := staging.Batch{Rows: []staging.Row{
		{ID: "a", Verdict: staging.New, AmountMinor: 300}, {ID: "b", Verdict: staging.New, AmountMinor: -100},
		{ID: "c", Verdict: staging.Duplicate}, {ID: "d", Verdict: staging.Unusable, Reason: "bad date"},
		{ID: "e", Verdict: staging.New, PossibleDuplicate: true, AmountMinor: -50}, {ID: "f", Verdict: staging.Verdict(99)},
	}}
	s := Summarize(b)
	if s.Income != 300 || s.Expense != 150 || s.Net != 150 || s.Duplicates != 1 || s.Invalid != 1 || s.Importable != 3 || s.Repeated != 1 || s.Errors["d"] != "bad date" || !s.RepeatedIDs["e"] || !s.DuplicateIDs["c"] {
		t.Fatal(s)
	}
	for _, amounts := range [][]int64{{math.MaxInt64, 1}, {math.MinInt64}, {-math.MaxInt64, -1, -1}} {
		b.Rows = nil
		for _, n := range amounts {
			b.Rows = append(b.Rows, staging.Row{Verdict: staging.New, AmountMinor: n})
		}
		if !Summarize(b).Overflow {
			t.Fatal(amounts)
		}
	}
}

func TestReceiptKeepsDiscountAndFirstValidDate(t *testing.T) {
	fallback := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	rows := []extract.Row{{Date: "bad", Amount: "10.00"}, {Date: "2026-03-01", Amount: "-2.00"}}
	r := Receipt(rows, " Shop\n Main ", "8.00")
	if r.Date != "2026-03-01" || r.Lines[1].Amount != "-2.00" || r.Merchant != "Shop  Main" {
		t.Fatal(r)
	}
	if residual, err := r.Residual(2); err != nil || residual != 0 {
		t.Fatal(residual, err)
	}
	if ReceiptDate(nil, fallback) != fallback {
		t.Fatal("fallback lost")
	}
}

func TestLocalSuggestionsKeepIdentityAndUserCategories(t *testing.T) {
	rows := []extract.Row{{ID: "a", Description: "STARBUCKS", Amount: "-3.00"}, {ID: "b", Description: "STARBUCKS", Amount: "-3.00", Category: "User"}, {ID: "c", Amount: "bad"}, {ID: "d", Description: "Unknown", Amount: "-1.00"}}
	updated := SuggestLocal(rows, nil, []domain.Category{{ID: "dining", Name: "Dining", Kind: domain.KindExpense}}, nil, 2)
	if updated[0].Category != "Dining" || updated[0].ID != "a" || updated[1].Category != "User" || updated[2].Category != "" || updated[3].Category != "" || rows[0].Category != "" {
		t.Fatal(updated)
	}
	m := New(rows)
	m.Suggest(nil, []domain.Category{{ID: "dining", Name: "Dining", Kind: domain.KindExpense}}, nil, 2)
	if m.Rows[0].Category != "Dining" {
		t.Fatal(m.Rows)
	}
}

func TestPresentationChoices(t *testing.T) {
	if RowError("a", map[string]string{"a": "staged"}, map[string]string{"a": "failed"}) != "failed" || RowError("a", map[string]string{"a": "staged"}, nil) != "staged" {
		t.Fatal("row reason")
	}
	for _, tt := range []struct{ noun, key string }{{"row", "imb.row"}, {"transaction", "imb.transaction"}, {"category", "imb.category"}, {"new transaction", "imb.newTransaction"}} {
		if CountKey(1, tt.noun) != tt.key+"One" || CountKey(0, tt.noun) != tt.key+"Many" {
			t.Fatal(tt)
		}
	}
}
