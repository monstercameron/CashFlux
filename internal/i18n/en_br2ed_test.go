// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestBR2EDBudgetRowCopy(t *testing.T) {
	for key, want := range map[string]string{
		"budgets.actionEditFormulasPlain":  "Calculate limit with formula",
		"budgets.pauseBudgetForMonth":      "Pause for one month",
		"budgets.actionCoverFromOtherMenu": "Cover from another budget",
		"budgets.limitTitleSuffix":         "limit",
		"budgets.baseLimitNameSuffix":      "base limit",
	} {
		if got := english[key]; got != want {
			t.Errorf("english[%q] = %q, want %q", key, got, want)
		}
	}
}
