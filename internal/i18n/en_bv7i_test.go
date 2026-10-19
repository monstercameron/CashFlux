// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestBV7ICatalogCopyUsesPlainSentenceCaseAndNoMiddleDotMeta(t *testing.T) {
	cases := map[string]string{
		"budgets.headlineAttention":       "Budgets needing attention: %d",
		"budgets.trackAllCategoryCount":   "Review untracked categories: %d",
		"budgets.fillAvg3":                "3-period average spend",
		"budgets.fillAvg6":                "6-period average spend",
		"budgets.rowPrimary":              "%s, %s",
		"budgets.rowSecondary":            "%s, %d%% used",
		"budgets.actionEditFormulasPlain": "Calculate limit with formula",
		"budgets.pauseBudgetForMonth":     "Pause for one month",
		"budgets.recurring.billFoldOne":   "%[2]s a month, %[1]d bill",
		"budgets.recurring.billFoldMany":  "%[2]s a month, %[1]d bills",
		"budgets.planYearStableHint":      "%d plan, %s",
		"budgets.smartNavLabel":           "Smart features",
		"budgets.directionSpend":          "Spending: stay under the amount",
		"bud2.sweep":                      "Sweep leftovers",
		"budgets.adjustAllNotANumber":     "Enter a number, like 5 or −10.",
	}
	for key, want := range cases {
		got, ok := english[key]
		if !ok {
			t.Errorf("missing %s", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
		if strings.Contains(got, "·") || got == strings.ToUpper(got) {
			t.Errorf("%s contains template copy %q", key, got)
		}
	}
}

func TestBV7ILimitEditAriaIncludesVisibleAmountAndBudget(t *testing.T) {
	got := DefaultBundle().T(English, "budgets.limitEditAria", "$205.20", "Groceries")
	want := "$205.20, change Groceries limit"
	if got != want {
		t.Fatalf("limit edit name = %q, want %q", got, want)
	}
}

func TestBV7IPermanentRaiseCopyNamesOneBudget(t *testing.T) {
	for _, key := range []string{"budgets.impactReachPermanent", "budgets.impactReachPermanentFunded"} {
		got := english[key]
		if strings.Contains(got, "budget limits") || !strings.Contains(got, "this budget's limit") {
			t.Errorf("%s does not describe one budget's limit: %q", key, got)
		}
	}
}

func TestBV7IAdjustExampleUsesMinusSign(t *testing.T) {
	for _, key := range []string{"budgets.adjustAllFieldHint", "budgets.adjustAllNotANumber"} {
		got := english[key]
		if strings.Contains(got, "-10") || !strings.Contains(got, "−10") {
			t.Errorf("%s uses the wrong minus sign: %q", key, got)
		}
	}
}
