// SPDX-License-Identifier: MIT

package pagination

import "testing"

func TestTotalPages(t *testing.T) {
	cases := []struct {
		total, size, want int
	}{
		{0, 50, 1}, // empty -> one (empty) page
		{1, 50, 1},
		{50, 50, 1},
		{51, 50, 2},
		{312, 50, 7}, // 6*50=300, +12 -> 7
		{100, 25, 4},
		{312, 0, 1},  // "all" -> one page
		{312, -3, 1}, // negative size treated as "all"
	}
	for _, c := range cases {
		if got := TotalPages(c.total, c.size); got != c.want {
			t.Errorf("TotalPages(%d,%d) = %d, want %d", c.total, c.size, got, c.want)
		}
	}
}

func TestClamp(t *testing.T) {
	// 312 items, 50/page -> 7 pages.
	cases := []struct {
		page, want int
	}{
		{-1, 1}, {0, 1}, {1, 1}, {4, 4}, {7, 7}, {8, 7}, {99, 7},
	}
	for _, c := range cases {
		if got := Clamp(c.page, 312, 50); got != c.want {
			t.Errorf("Clamp(%d, 312, 50) = %d, want %d", c.page, got, c.want)
		}
	}
}

func TestBounds(t *testing.T) {
	cases := []struct {
		page, total, size, start, end int
	}{
		{1, 312, 50, 0, 50},
		{2, 312, 50, 50, 100},
		{7, 312, 50, 300, 312}, // last partial page
		{8, 312, 50, 300, 312}, // out of range -> clamped to last
		{1, 312, 0, 0, 312},    // "all"
		{1, 0, 50, 0, 0},       // empty
	}
	for _, c := range cases {
		if s, e := Bounds(c.page, c.total, c.size); s != c.start || e != c.end {
			t.Errorf("Bounds(%d,%d,%d) = (%d,%d), want (%d,%d)", c.page, c.total, c.size, s, e, c.start, c.end)
		}
	}
}

func TestSlice(t *testing.T) {
	items := make([]int, 312)
	for i := range items {
		items[i] = i
	}
	p1 := Slice(items, 1, 50)
	if len(p1) != 50 || p1[0] != 0 || p1[49] != 49 {
		t.Errorf("page 1 = len %d [%d..%d]", len(p1), p1[0], p1[len(p1)-1])
	}
	last := Slice(items, 7, 50)
	if len(last) != 12 || last[0] != 300 || last[11] != 311 {
		t.Errorf("page 7 = len %d", len(last))
	}
	all := Slice(items, 1, AllSize)
	if len(all) != 312 {
		t.Errorf("all = len %d, want 312", len(all))
	}
}

func TestWindow(t *testing.T) {
	cases := []struct {
		page, total, size, from, to int
	}{
		{1, 312, 50, 1, 50},
		{2, 312, 50, 51, 100},
		{7, 312, 50, 301, 312},
		{1, 0, 50, 0, 0}, // empty
		{1, 312, 0, 1, 312},
	}
	for _, c := range cases {
		if f, to := Window(c.page, c.total, c.size); f != c.from || to != c.to {
			t.Errorf("Window(%d,%d,%d) = (%d,%d), want (%d,%d)", c.page, c.total, c.size, f, to, c.from, c.to)
		}
	}
}
