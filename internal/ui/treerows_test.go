package ui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// IndentPx
// ---------------------------------------------------------------------------

func TestIndentPx(t *testing.T) {
	tests := []struct {
		depth int
		want  string
	}{
		{depth: 0, want: "0px"},
		{depth: -1, want: "0px"},
		{depth: -99, want: "0px"},
		{depth: 1, want: "16px"},
		{depth: 2, want: "32px"},
		{depth: 3, want: "48px"},
		{depth: 5, want: "80px"},
		{depth: 6, want: "96px"},
		{depth: 10, want: "160px"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := IndentPx(tc.depth)
			if got != tc.want {
				t.Errorf("IndentPx(%d) = %q; want %q", tc.depth, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IndentLabel
// ---------------------------------------------------------------------------

func TestIndentLabel(t *testing.T) {
	const nbsp = " "

	tests := []struct {
		depth     int
		wantCount int // number of NBSP characters expected
	}{
		{depth: 0, wantCount: 0},
		{depth: -5, wantCount: 0},
		{depth: 1, wantCount: 3},
		{depth: 2, wantCount: 6},
		{depth: 3, wantCount: 9},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := IndentLabel(tc.depth)
			count := strings.Count(got, nbsp)
			if count != tc.wantCount {
				t.Errorf("IndentLabel(%d) NBSP count = %d; want %d (got %q)",
					tc.depth, count, tc.wantCount, got)
			}
		})
	}
}

// IndentLabel output can be prepended to a name string.
func TestIndentLabelConcat(t *testing.T) {
	label := IndentLabel(2) + "Child"
	if !strings.HasSuffix(label, "Child") {
		t.Errorf("expected suffix 'Child', got %q", label)
	}
}

// ---------------------------------------------------------------------------
// ClampDepth
// ---------------------------------------------------------------------------

func TestClampDepth(t *testing.T) {
	tests := []struct {
		depth int
		want  int
	}{
		{depth: -10, want: 0},
		{depth: -1, want: 0},
		{depth: 0, want: 0},
		{depth: 1, want: 1},
		{depth: MaxIndentDepth, want: MaxIndentDepth},
		{depth: MaxIndentDepth + 1, want: MaxIndentDepth},
		{depth: 100, want: MaxIndentDepth},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := ClampDepth(tc.depth)
			if got != tc.want {
				t.Errorf("ClampDepth(%d) = %d; want %d", tc.depth, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// itoa (package-private, tested via IndentPx)
// ---------------------------------------------------------------------------

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{16, "16"},
		{100, "100"},
		{160, "160"},
		{999, "999"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := itoa(tc.n)
			if got != tc.want {
				t.Errorf("itoa(%d) = %q; want %q", tc.n, got, tc.want)
			}
		})
	}
}
