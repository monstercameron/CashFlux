package dashlayout

import "testing"

func TestClampSpan(t *testing.T) {
	const max = 4
	cases := []struct {
		name string
		v    int
		want int
	}{
		{"below min collapses to 1", 0, 1},
		{"negative collapses to 1", -3, 1},
		{"min stays", 1, 1},
		{"mid stays", 3, 3},
		{"max stays", 4, 4},
		{"above max collapses to max", 9, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClampSpan(c.v, max); got != c.want {
				t.Errorf("ClampSpan(%d, %d) = %d, want %d", c.v, max, got, c.want)
			}
		})
	}
}

func TestCycleSpan(t *testing.T) {
	const max = 4
	cases := []struct {
		name   string
		cur    int
		shrink bool
		want   int
	}{
		{"grow steps up", 1, false, 2},
		{"grow mid", 3, false, 4},
		{"grow at max wraps to 1", 4, false, 1},
		{"shrink steps down", 3, true, 2},
		{"shrink at 1 stays", 1, true, 1},
		{"shrink from 2 to 1", 2, true, 1},
		{"shrink at max steps down (no wrap)", 4, true, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CycleSpan(c.cur, max, c.shrink); got != c.want {
				t.Errorf("CycleSpan(%d, %d, %v) = %d, want %d", c.cur, max, c.shrink, got, c.want)
			}
		})
	}
}
