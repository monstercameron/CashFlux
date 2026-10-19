// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBC5B9BudgetRowCopyIsCatalogued(t *testing.T) {
	if got, want := english["bc5b9.budgetRowOf"], "of"; got != want {
		t.Fatalf("budget row separator = %q, want %q", got, want)
	}
	for key, want := range map[string]string{
		"budgets.addedCount.one":  "1 budget",
		"budgets.addedCount.many": "%d budgets",
	} {
		if got := english[key]; got != want {
			t.Errorf("English copy %q = %q, want %q", key, got, want)
		}
	}
}
