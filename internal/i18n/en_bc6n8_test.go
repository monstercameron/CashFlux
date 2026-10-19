// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBC6N8AutoBudgetErrorCopyIsRegistered(t *testing.T) {
	got := DefaultBundle().T(English, "budgets.autoErrorGeneric")
	if got != "Auto budget could not be calculated. Try again." {
		t.Fatalf("generic auto-budget error = %q", got)
	}
}

func TestBC6N8AllocationErrorCopyIsRegistered(t *testing.T) {
	got := DefaultBundle().T(English, "budgets.allocationError")
	if got != "We could not calculate this income split." {
		t.Fatalf("allocation error = %q", got)
	}
}

func TestBC6N8BudgetCountPluralFormsAreAvailable(t *testing.T) {
	b := DefaultBundle()
	if got := b.T(English, "bc4b4.budgetCount.one"); got != "1 budget" {
		t.Fatalf("singular budget count = %q", got)
	}
	if got := b.T(English, "bc4b4.budgetCount.many", 2); got != "2 budgets" {
		t.Fatalf("plural budget count = %q", got)
	}
}
