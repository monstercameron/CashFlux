package dashlayout

import (
	"reflect"
	"testing"
)

func ids(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func reversed(items []Item) []Item {
	out := append([]Item(nil), items...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func TestModeValid(t *testing.T) {
	for _, m := range []Mode{ModeCustom, ModeAutoDefault, ModeAutoImportance} {
		if !m.Valid() {
			t.Errorf("%q should be valid", m)
		}
	}
	if Mode("nonsense").Valid() {
		t.Error(`"nonsense" should be invalid`)
	}
}

func TestArrangeCustomIsNoOp(t *testing.T) {
	in := reversed(DefaultItems())
	got := Arrange(in, ModeCustom)
	if !reflect.DeepEqual(ids(got), ids(in)) {
		t.Errorf("custom reordered the items: got %v", ids(got))
	}
}

func TestArrangeAutoDefaultRestoresCanonicalOrder(t *testing.T) {
	// Any starting order must come back to the DefaultItems order.
	got := Arrange(reversed(DefaultItems()), ModeAutoDefault)
	if !reflect.DeepEqual(ids(got), ids(DefaultItems())) {
		t.Errorf("auto-default order = %v, want %v", ids(got), ids(DefaultItems()))
	}
}

func TestArrangeAutoImportanceSortsHighFirstThenCanonical(t *testing.T) {
	// kpi-income (canonically after kpi-networth) is made most important, so it
	// must lead; the two unrated tiles keep canonical order (networth before
	// spending) behind it.
	in := []Item{
		{ID: "kpi-networth", ColSpan: 1, RowSpan: 1, Importance: 0},
		{ID: "kpi-income", ColSpan: 1, RowSpan: 1, Importance: 5},
		{ID: "kpi-spending", ColSpan: 1, RowSpan: 1, Importance: 0},
	}
	got := ids(Arrange(in, ModeAutoImportance))
	want := []string{"kpi-income", "kpi-networth", "kpi-spending"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("auto-importance order = %v, want %v", got, want)
	}
}

func TestArrangeAutoImportanceTiesAreStableByCanonicalOrder(t *testing.T) {
	// All equal importance → fall back to canonical order regardless of input.
	in := reversed([]Item{
		{ID: "kpi-networth", Importance: 2},
		{ID: "kpi-income", Importance: 2},
		{ID: "kpi-spending", Importance: 2},
	})
	got := ids(Arrange(in, ModeAutoImportance))
	want := []string{"kpi-networth", "kpi-income", "kpi-spending"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tie order = %v, want canonical %v", got, want)
	}
}

func TestArrangeDoesNotMutateInputOrSpans(t *testing.T) {
	in := []Item{
		{ID: "recent", ColSpan: 2, RowSpan: 2, Importance: 1},
		{ID: "kpi-networth", ColSpan: 1, RowSpan: 1, Importance: 9},
	}
	before := append([]Item(nil), in...)
	out := Arrange(in, ModeAutoImportance)
	if !reflect.DeepEqual(in, before) {
		t.Errorf("input was mutated: %+v", in)
	}
	// Spans are preserved (auto-layout only reorders, never resizes).
	for _, it := range out {
		switch it.ID {
		case "recent":
			if it.ColSpan != 2 || it.RowSpan != 2 {
				t.Errorf("recent spans changed: %+v", it)
			}
		case "kpi-networth":
			if it.ColSpan != 1 || it.RowSpan != 1 {
				t.Errorf("kpi-networth spans changed: %+v", it)
			}
		}
	}
}

func TestArrangeUnknownIdsSortAfterKnown(t *testing.T) {
	in := []Item{
		{ID: "custom-b"},
		{ID: "kpi-income"},
		{ID: "custom-a"},
		{ID: "kpi-networth"},
	}
	got := ids(Arrange(in, ModeAutoDefault))
	// Known ids first in canonical order, then unknowns in their original order.
	want := []string{"kpi-networth", "kpi-income", "custom-b", "custom-a"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestSetImportanceAndImportanceOf(t *testing.T) {
	in := []Item{{ID: "a"}, {ID: "b", Importance: 1}}
	out := SetImportance(in, "b", 5)
	if ImportanceOf(out, "b") != 5 {
		t.Errorf("ImportanceOf(b) = %d, want 5", ImportanceOf(out, "b"))
	}
	if ImportanceOf(out, "a") != 0 {
		t.Errorf("ImportanceOf(a) = %d, want 0", ImportanceOf(out, "a"))
	}
	if in[1].Importance != 1 {
		t.Errorf("input was mutated: %+v", in)
	}
	// Unknown id is a no-op copy; ImportanceOf of an unknown id is 0.
	if got := SetImportance(in, "zzz", 9); !reflect.DeepEqual(got, in) {
		t.Errorf("unknown-id SetImportance changed items: %+v", got)
	}
	if ImportanceOf(in, "zzz") != 0 {
		t.Error("ImportanceOf of unknown id should be 0")
	}
}

func TestArrangeThenPackHasNoOverlap(t *testing.T) {
	for _, mode := range []Mode{ModeCustom, ModeAutoDefault, ModeAutoImportance} {
		layout := Pack(Arrange(DefaultItems(), mode), 4)
		seen := map[[2]int]string{}
		for _, p := range layout {
			for r := p.Row; r < p.Row+p.RowSpan; r++ {
				for c := p.Col; c < p.Col+p.ColSpan; c++ {
					if other, ok := seen[[2]int{r, c}]; ok {
						t.Fatalf("mode %s: %s overlaps %s at (%d,%d)", mode, p.ID, other, r, c)
					}
					seen[[2]int{r, c}] = p.ID
				}
			}
		}
	}
}
