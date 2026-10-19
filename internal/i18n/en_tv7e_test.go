// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestTV7ETransactionsCopy(t *testing.T) {
	bundle := DefaultBundle()
	cases := []struct {
		key  string
		args []any
		want string
	}{
		{key: "r1c.reviewCallout", args: []any{249}, want: "Review 249"},
		{key: "r2a.upcomingOne", args: []any{1, "$220.00"}, want: "1 bill due: $220.00"},
		{key: "r2a.upcomingMany", args: []any{3, "$707.00"}, want: "3 bills due: $707.00"},
		{key: "tc3w13.futureSummary", args: []any{"November 2026", "0", "9", "$707.00"}, want: "November 2026, 0 posted, 9 scheduled, $707.00 due"},
		{key: "transactions.savedImpact", args: []any{"Checking", "$1,200.00"}, want: "Saved. Checking is now $1,200.00."},
		{key: "transactions.noMatchKeepsFilters", want: "Clearing the search keeps your other filters."},
		{key: "review.title", want: "Review"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := bundle.T(English, tc.key, tc.args...); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
