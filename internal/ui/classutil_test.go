package ui

import "testing"

func TestJoinClass(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   string
	}{
		{
			name:   "empty input",
			tokens: nil,
			want:   "",
		},
		{
			name:   "single token",
			tokens: []string{"btn"},
			want:   "btn",
		},
		{
			name:   "multiple tokens",
			tokens: []string{"btn", "btn-del"},
			want:   "btn btn-del",
		},
		{
			name:   "empty tokens omitted",
			tokens: []string{"btn", "", "btn-del", ""},
			want:   "btn btn-del",
		},
		{
			name:   "whitespace-only tokens omitted",
			tokens: []string{"  ", "row", "   "},
			want:   "row",
		},
		{
			name:   "tokens with surrounding whitespace trimmed",
			tokens: []string{"  row  ", " row-main "},
			want:   "row row-main",
		},
		{
			name:   "all empty",
			tokens: []string{"", "", ""},
			want:   "",
		},
		{
			name:   "danger modifier",
			tokens: []string{"btn", "btn-del", ""},
			want:   "btn btn-del",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := JoinClass(tc.tokens...)
			if got != tc.want {
				t.Errorf("JoinClass(%v) = %q; want %q", tc.tokens, got, tc.want)
			}
		})
	}
}
