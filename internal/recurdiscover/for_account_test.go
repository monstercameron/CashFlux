// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"reflect"
	"testing"
)

func TestResultForAccount(t *testing.T) {
	res := Result{Candidates: []Candidate{
		{Payee: "a1", AccountID: "a"}, {Payee: "b1", AccountID: "b"}, {Payee: "a2", AccountID: "a"},
		{Payee: "a3", AccountID: "a"}, {Payee: "b2", AccountID: "b"},
	}}
	payees := func(cs []Candidate) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Payee)
		}
		return out
	}
	cases := []struct {
		name  string
		acct  string
		limit int
		want  []string
	}{
		{"filters to the account", "b", 0, []string{"b1", "b2"}},
		{"cap keeps discovery order", "a", 2, []string{"a1", "a2"}},
		{"cap above the matches returns them all", "a", 10, []string{"a1", "a2", "a3"}},
		{"negative limit is uncapped", "a", -1, []string{"a1", "a2", "a3"}},
		{"unknown account", "z", 3, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := payees(res.ForAccount(tc.acct, tc.limit)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ForAccount(%q, %d) = %v, want %v", tc.acct, tc.limit, got, tc.want)
			}
		})
	}
}
