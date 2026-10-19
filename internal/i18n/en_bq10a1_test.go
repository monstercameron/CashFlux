// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestBQ10A1ImpactCopyIsOneCompleteSentencePerReach(t *testing.T) {
	for _, key := range []string{
		"budgets.impactReachThisPeriod",
		"budgets.impactReachThisPeriodFunded",
		"budgets.impactReachThisAndLater",
		"budgets.impactReachThisAndLaterFunded",
		"budgets.impactReachThisAndNext",
		"budgets.impactReachThisAndNextFunded",
		"budgets.impactReachFutureOnly",
		"budgets.impactReachPermanent",
		"budgets.impactReachPermanentFunded",
		"budgets.impactReachDelete",
	} {
		copy, ok := english[key]
		if !ok || strings.TrimSpace(copy) == "" || !strings.HasSuffix(copy, ".") {
			t.Errorf("English[%q] = %q, want a complete sentence", key, copy)
		}
		if strings.Count(copy, ".") != 1 {
			t.Errorf("English[%q] = %q, want one sentence", key, copy)
		}
	}
	if strings.Contains(english["budgets.impactReachThisPeriod"], "budgets you selected") {
		t.Fatal("unfunded period-only copy claims money comes from selected budgets")
	}
}
