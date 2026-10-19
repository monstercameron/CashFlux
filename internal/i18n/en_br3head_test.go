// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestBR3HeadLabelsUseSentenceCaseAndPlainCopy(t *testing.T) {
	for _, key := range []string{
		"budgets.allocCap", "budgets.allocCapHist", "budgets.recurring.title",
		"budgets.recurring.allCategoriesTitle", "budgets.tmplOr", "budgetMetrics.perDay",
		"budgetMetrics.daysLeft", "budgetMetrics.elapsed", "budgets.driversShow", "budgets.br3.smartNavLabel",
	} {
		value := english[key]
		if value == "" {
			t.Errorf("missing copy for %s", key)
			continue
		}
		if strings.Contains(value, "→") || value == strings.ToUpper(value) {
			t.Errorf("%s contains template copy %q", key, value)
		}
	}
}
