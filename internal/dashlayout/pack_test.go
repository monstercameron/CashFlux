// SPDX-License-Identifier: MIT

package dashlayout

import "testing"

// cellsOf returns the set of (row,col) cells a placement occupies, for overlap
// and coverage assertions.
func cellsOf(p Placement) [][2]int {
	var out [][2]int
	for dr := 0; dr < p.RowSpan; dr++ {
		for dc := 0; dc < p.ColSpan; dc++ {
			out = append(out, [2]int{p.Row + dr, p.Col + dc})
		}
	}
	return out
}

func assertNoOverlap(t *testing.T, l Layout, cols int) {
	t.Helper()
	seen := map[[2]int]string{}
	for _, p := range l {
		if p.Col < 1 || p.Col+p.ColSpan-1 > cols {
			t.Errorf("%s out of bounds: col %d span %d (cols=%d)", p.ID, p.Col, p.ColSpan, cols)
		}
		for _, c := range cellsOf(p) {
			if other, ok := seen[c]; ok {
				t.Errorf("overlap at %v between %s and %s", c, other, p.ID)
			}
			seen[c] = p.ID
		}
	}
}

func TestPackDefaultReproducesArrangement(t *testing.T) {
	got := Pack(DefaultItems(), 4)
	want := map[string][2]int{ // id -> {col, row} (1-based)
		"attention":    {1, 1}, // full-width digest at the top
		"kpi-networth": {1, 2}, "kpi-income": {2, 2}, "kpi-spending": {3, 2}, "kpi-liabilities": {4, 2},
		"recent": {1, 3}, "budgets": {3, 3}, "trend": {4, 3},
		"goals": {1, 5}, "todo": {2, 5}, "accounts": {3, 5},
		"cashflow": {1, 6}, "bills": {3, 6},
		"savings": {1, 7}, "breakdown": {3, 7},
		"freshness":    {1, 8},
		"highlight":    {3, 8},
		"smart-digest": {1, 9},
	}
	for _, p := range got {
		w, ok := want[p.ID]
		if !ok {
			t.Errorf("unexpected widget %s", p.ID)
			continue
		}
		if p.Col != w[0] || p.Row != w[1] {
			t.Errorf("%s at col %d row %d, want col %d row %d", p.ID, p.Col, p.Row, w[0], w[1])
		}
	}
	assertNoOverlap(t, got, 4)
}

func TestPackClampsAndNoOverlapWithMixedSpans(t *testing.T) {
	items := []Item{
		{ID: "wide", ColSpan: 9, RowSpan: 1}, // clamp to 4
		{ID: "tall", ColSpan: 1, RowSpan: 2},
		{ID: "zero", ColSpan: 0, RowSpan: 0}, // clamp to 1x1
		{ID: "block", ColSpan: 3, RowSpan: 2},
	}
	got := Pack(items, 4)
	assertNoOverlap(t, got, 4)
	by := map[string]Placement{}
	for _, p := range got {
		by[p.ID] = p
	}
	if by["wide"].ColSpan != 4 {
		t.Errorf("wide colspan = %d, want clamped to 4", by["wide"].ColSpan)
	}
	if by["zero"].ColSpan != 1 || by["zero"].RowSpan != 1 {
		t.Errorf("zero span = %dx%d, want clamped to 1x1", by["zero"].ColSpan, by["zero"].RowSpan)
	}
	// wide fills row 1; tall starts row 2 col 1 (rows 2-3); zero (1x1) backfills
	// the earliest gap at row 2 col 2; block (3x2) doesn't fit beside tall+zero
	// in row 2, so it lands at row 4.
	if by["tall"].Row != 2 || by["tall"].Col != 1 {
		t.Errorf("tall at col %d row %d, want col 1 row 2", by["tall"].Col, by["tall"].Row)
	}
}

func TestPackFirstFitBackfillsGaps(t *testing.T) {
	// A 1x1 after a 4-wide leaves a row; a later small tile fills the earliest gap.
	items := []Item{
		{ID: "a", ColSpan: 3, RowSpan: 1},
		{ID: "b", ColSpan: 2, RowSpan: 1}, // doesn't fit in row 1 (only 1 col left) -> row 2
		{ID: "c", ColSpan: 1, RowSpan: 1}, // backfills row 1 col 4
	}
	got := Pack(items, 4)
	by := map[string]Placement{}
	for _, p := range got {
		by[p.ID] = p
	}
	if by["c"].Row != 1 || by["c"].Col != 4 {
		t.Errorf("c at col %d row %d, want col 4 row 1 (backfilled)", by["c"].Col, by["c"].Row)
	}
	if by["b"].Row != 2 || by["b"].Col != 1 {
		t.Errorf("b at col %d row %d, want col 1 row 2", by["b"].Col, by["b"].Row)
	}
	assertNoOverlap(t, got, 4)
}

func TestMoveReorders(t *testing.T) {
	items := []Item{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	got := Move(items, "d", 1)
	want := []string{"a", "d", "b", "c"}
	for i, w := range want {
		if got[i].ID != w {
			t.Fatalf("Move(d,1) = %v, want %v", idsOf(got), want)
		}
	}
	// Original untouched.
	if items[3].ID != "d" {
		t.Errorf("Move mutated input: %v", idsOf(items))
	}
}

func TestMoveClampsAndUnknownNoop(t *testing.T) {
	items := []Item{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	if got := Move(items, "a", 99); idsOf(got)[2] != "a" {
		t.Errorf("Move(a, 99) = %v, want a moved to end", idsOf(got))
	}
	if got := Move(items, "nope", 0); idsOf(got)[0] != "a" {
		t.Errorf("Move(unknown) changed order: %v", idsOf(got))
	}
}

func TestResizeItemClamps(t *testing.T) {
	items := []Item{{ID: "a", ColSpan: 1, RowSpan: 1}}
	got := ResizeItem(items, "a", 3, 0)
	if got[0].ColSpan != 3 || got[0].RowSpan != 1 {
		t.Errorf("ResizeItem = %dx%d, want 3x1 (rowspan clamped)", got[0].ColSpan, got[0].RowSpan)
	}
	if items[0].ColSpan != 1 {
		t.Error("ResizeItem mutated input")
	}
}

func idsOf(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}
