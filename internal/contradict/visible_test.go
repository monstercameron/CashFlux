// SPDX-License-Identifier: MIT

package contradict

import "testing"

func TestVisibleHidesDismissedFindingsOnly(t *testing.T) {
	fs := []Finding{{Key: "a"}, {Key: "b"}, {Key: "c"}}
	tests := []struct {
		name      string
		dismissed map[string]bool
		want      []string
	}{
		{"nil set hides nothing", nil, []string{"a", "b", "c"}},
		{"empty set hides nothing", map[string]bool{}, []string{"a", "b", "c"}},
		{"one dismissed", map[string]bool{"b": true}, []string{"a", "c"}},
		{"every dismissed", map[string]bool{"a": true, "b": true, "c": true}, []string{}},
		{"unknown key is ignored", map[string]bool{"zzz": true}, []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Visible(fs, tt.dismissed)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i, f := range got {
				if f.Key != tt.want[i] {
					t.Errorf("got[%d].Key = %q, want %q", i, f.Key, tt.want[i])
				}
			}
		})
	}
}
