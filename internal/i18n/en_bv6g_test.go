// SPDX-License-Identifier: MIT

package i18n

import (
	"fmt"
	"strings"
	"testing"
)

func TestBV6GEnglishCopyHasCompleteFormattingAndPlainActionLabels(t *testing.T) {
	args := map[string][]any{
		"bv6g.monthclose.leftItemRollover": {"Groceries", "$25.00"},
		"bv6g.monthclose.leftItemFresh":    {"Groceries", "$25.00"},
		"bv6g.monthclose.leftTotal":        {"$275.00", 2},
		"bv6g.monthclose.incomeLive":       {"$5,900.00", "$7,063.00"},
		"bv6g.cover.sourceBudget":          {"Dining", "$300.00", "$275.00"},
		"bv6g.cover.apply":                 {"$37.30"},
		"bv6g.raise.apply":                 {"$37.30"},
	}
	for key, values := range args {
		copy, ok := english[key]
		if !ok {
			t.Errorf("missing English key %q", key)
			continue
		}
		formatted := fmt.Sprintf(copy, values...)
		if strings.Contains(formatted, "%!") {
			t.Errorf("%s has a formatting mismatch: %q", key, formatted)
		}
	}
	for _, key := range []string{"bv6g.adjust.scopeLabel", "bv6g.adjust.thisPeriod", "bv6g.adjust.allPeriods"} {
		if english[key] == "" {
			t.Errorf("missing scope label %q", key)
		}
	}
	if strings.Contains(english["bv6g.monthclose.leftItemRollover"], "·") || strings.Contains(english["bv6g.monthclose.leftItemFresh"], "·") {
		t.Fatal("month review copy must not join metadata with a middle dot")
	}
}
