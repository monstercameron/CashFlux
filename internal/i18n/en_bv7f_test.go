// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBV7FDelayedBudgetSaveCopyIsPlainEnglish(t *testing.T) {
	got := DefaultBundle().T(English, "budgets.limitSaving")
	if got != "Saving your budget change…" {
		t.Fatalf("delayed save copy = %q", got)
	}
}
