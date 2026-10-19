// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBC4B6AdjustAllBudgetCounts(t *testing.T) {
	for _, tc := range []struct {
		key  string
		args []any
		want string
	}{
		{key: "budgets.countOne", want: "1 budget"},
		{key: "budgets.countMany", args: []any{4}, want: "4 budgets"},
	} {
		if got := DefaultBundle().T(English, tc.key, tc.args...); got != tc.want {
			t.Errorf("English %q = %q, want %q", tc.key, got, tc.want)
		}
	}
}
