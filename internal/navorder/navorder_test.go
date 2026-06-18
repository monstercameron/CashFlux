package navorder

import (
	"reflect"
	"testing"
)

func TestMove(t *testing.T) {
	base := []string{"a", "b", "c", "d"}
	tests := []struct {
		name    string
		id      string
		toIndex int
		want    []string
	}{
		{"forward", "a", 2, []string{"b", "c", "a", "d"}},
		{"backward", "d", 0, []string{"d", "a", "b", "c"}},
		{"same index is a no-op", "b", 1, []string{"a", "b", "c", "d"}},
		{"clamp high", "a", 99, []string{"b", "c", "d", "a"}},
		{"clamp low", "c", -5, []string{"c", "a", "b", "d"}},
		{"unknown id unchanged", "z", 1, []string{"a", "b", "c", "d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Move(base, tt.id, tt.toIndex)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Move(%v, %q, %d) = %v, want %v", base, tt.id, tt.toIndex, got, tt.want)
			}
			if !reflect.DeepEqual(base, []string{"a", "b", "c", "d"}) {
				t.Errorf("input was mutated: %v", base)
			}
		})
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name           string
		saved, current []string
		want           []string
	}{
		{"empty saved keeps current order", nil, []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"saved reorders current", []string{"c", "a", "b"}, []string{"a", "b", "c"}, []string{"c", "a", "b"}},
		{"new current id appends after saved", []string{"b", "a"}, []string{"a", "b", "c"}, []string{"b", "a", "c"}},
		{"saved id absent from current is dropped", []string{"x", "b", "a"}, []string{"a", "b"}, []string{"b", "a"}},
		{"duplicates in saved ignored", []string{"a", "a", "b"}, []string{"a", "b"}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply(tt.saved, tt.current)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Apply(%v, %v) = %v, want %v", tt.saved, tt.current, got, tt.want)
			}
		})
	}
}

func TestApplyThenMoveRoundTrips(t *testing.T) {
	current := []string{"dashboard", "accounts", "transactions", "budgets"}
	// Move budgets to the front, then Apply that saved order to the same list.
	saved := Move(current, "budgets", 0)
	got := Apply(saved, current)
	want := []string{"budgets", "dashboard", "accounts", "transactions"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %v, want %v", got, want)
	}
}
