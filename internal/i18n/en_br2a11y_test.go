// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestBudgetAccessibleNamesStartWithVisibleText(t *testing.T) {
	for key, visible := range map[string]string{
		"budgets.basisButtonTitle": "Budget income",
		"budgets.seeSpendTitle":    "See where it went",
		"scope.viewAllTitle":       "View all",
	} {
		if name := english[key]; !strings.HasPrefix(name, visible) {
			t.Errorf("English[%q] = %q, want prefix %q", key, name, visible)
		}
	}
}
