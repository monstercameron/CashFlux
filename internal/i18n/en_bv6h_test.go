// SPDX-License-Identifier: MIT

package i18n

import (
	"fmt"
	"strings"
	"testing"
)

func TestBV6HEnglishCopyUsesPlainNamesAndCorrectGrammar(t *testing.T) {
	for key, want := range map[string]string{
		"budgets.trackAllCategoryCount":   "Review untracked categories: %d",
		"budgets.recurring.excludedOne":   "%d bill excluded",
		"budgets.recurring.excludedMany":  "%d bills excluded",
		"budgets.attention.one":           "%d budget needs attention",
		"budgets.attention.many":          "%d budgets need attention",
		"budgets.reviewMonth.oneUnusual":  "%d unusual month excluded",
		"budgets.reviewMonth.manyUnusual": "%d unusual months excluded",
	} {
		if got := english[key]; got != want {
			t.Errorf("English copy %q = %q, want %q", key, got, want)
		}
	}
	negativeDetail := fmt.Sprintf(english["smart.p5.detailNegative"], "$500.00", "-$50.00")
	if strings.Contains(negativeDetail, "%!") {
		t.Fatalf("negative planning detail has mismatched format arguments: %q", negativeDetail)
	}
}
