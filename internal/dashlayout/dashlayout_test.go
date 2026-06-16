package dashlayout

import "testing"

func TestAxisCSS(t *testing.T) {
	tests := []struct {
		p        Placement
		col, row string
	}{
		{Placement{Col: 1, Row: 2, ColSpan: 1, RowSpan: 1}, "1", "2"},
		{Placement{Col: 1, Row: 3, ColSpan: 2, RowSpan: 2}, "1 / span 2", "3 / span 2"},
		{Placement{Col: 3, Row: 5, ColSpan: 2, RowSpan: 1}, "3 / span 2", "5"},
	}
	for _, tt := range tests {
		if got := tt.p.GridColumn(); got != tt.col {
			t.Errorf("GridColumn(%+v) = %q, want %q", tt.p, got, tt.col)
		}
		if got := tt.p.GridRow(); got != tt.row {
			t.Errorf("GridRow(%+v) = %q, want %q", tt.p, got, tt.row)
		}
	}
}

func TestDefaultHasUniqueIDs(t *testing.T) {
	l := Default()
	seen := map[string]bool{}
	for _, p := range l {
		if seen[p.ID] {
			t.Errorf("duplicate id %q", p.ID)
		}
		seen[p.ID] = true
	}
	if len(l) != 14 {
		t.Errorf("default layout has %d widgets, want 14", len(l))
	}
}

func TestSwap(t *testing.T) {
	l := Default()
	a, _ := l.Get("kpi-networth") // col1 row2
	b, _ := l.Get("recent")       // col1 row3 span2x2

	swapped := l.Swap("kpi-networth", "recent")

	na, _ := swapped.Get("kpi-networth")
	nb, _ := swapped.Get("recent")
	if na.Col != b.Col || na.Row != b.Row || na.ColSpan != b.ColSpan || na.RowSpan != b.RowSpan {
		t.Errorf("kpi-networth should take recent's cell, got %+v", na)
	}
	if nb.Col != a.Col || nb.Row != a.Row || nb.ColSpan != a.ColSpan || nb.RowSpan != a.RowSpan {
		t.Errorf("recent should take kpi-networth's cell, got %+v", nb)
	}
	// Original layout is untouched (immutability).
	if oa, _ := l.Get("kpi-networth"); oa != a {
		t.Error("Swap mutated the original layout")
	}
}

func TestSwapUnknownNoop(t *testing.T) {
	l := Default()
	out := l.Swap("kpi-networth", "nope")
	for i := range l {
		if l[i] != out[i] {
			t.Errorf("Swap with unknown id changed placement %d", i)
		}
	}
}

func TestResizeClamps(t *testing.T) {
	l := Default().Resize("goals", 3, 0)
	p, _ := l.Get("goals")
	if p.ColSpan != 3 || p.RowSpan != 1 {
		t.Errorf("Resize = %dx%d, want 3x1 (rowspan clamped)", p.ColSpan, p.RowSpan)
	}
}
