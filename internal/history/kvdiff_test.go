// SPDX-License-Identifier: MIT

package history

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestScalarMapDiffKeys(t *testing.T) {
	cases := []struct {
		name   string
		before string
		after  string
		want   []string
		wantOK bool
	}{
		{"no change", `{"a":"1","b":"2"}`, `{"a":"1","b":"2"}`, nil, true},
		{"value changed", `{"a":"1","b":"2"}`, `{"a":"1","b":"3"}`, []string{"b"}, true},
		{"key added", `{"a":"1"}`, `{"a":"1","c":"9"}`, []string{"c"}, true},
		{"key removed", `{"a":"1","b":"2"}`, `{"a":"1"}`, []string{"b"}, true},
		{"mixed", `{"a":"1","b":"2"}`, `{"b":"3","c":"4"}`, []string{"a", "b", "c"}, true},
		{"not an object", `[1,2]`, `{"a":"1"}`, nil, false},
		{"corrupt after", `{"a":"1"}`, `{oops`, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ScalarMapDiffKeys(json.RawMessage(tc.before), json.RawMessage(tc.after))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("keys = %v, want %v", got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("keys = %v, want %v", got, want)
				}
			}
		})
	}
}
