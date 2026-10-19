// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBQ10BBudgetTaskKeysFormatBudgetName(t *testing.T) {
	bundle := DefaultBundle()
	for _, tc := range []struct {
		key  string
		want string
	}{
		{key: "tasks.budgetOverLimit", want: "Budget over limit: Groceries"},
		{key: "tasks.budgetSavingShortfall", want: "Savings target needs funding: Groceries"},
		{key: "budgets.goalFundedOrdinaryOver", want: "Even without goal-funded spending, this budget is $5.00 over its limit."},
	} {
		arg := any("Groceries")
		if tc.key == "budgets.goalFundedOrdinaryOver" {
			arg = "$5.00"
		}
		if got := bundle.T(English, tc.key, arg); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.key, got, tc.want)
		}
	}
}
