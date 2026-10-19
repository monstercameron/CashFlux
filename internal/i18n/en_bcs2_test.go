// SPDX-License-Identifier: MIT

package i18n

import (
	"fmt"
	"strings"
	"testing"
)

func TestBCS2MoneyFlowCopyFormatsAndKeepsHistoryPromise(t *testing.T) {
	for _, tc := range []struct {
		key  string
		args []any
	}{
		{"bcs2.adjust.fromPeriod", nil},
		{"bcs2.adjust.fromHint", nil},
		{"bcs2.adjust.applyFrom", nil},
		{"bcs2.adjust.appliedFrom", []any{"2 budgets", "10%"}},
		{"bcs2.copy.applied", []any{"1 budget"}},
		{"bcs2.copy.intro", nil},
	} {
		value := fmt.Sprintf(english[tc.key], tc.args...)
		if value == "" || strings.Contains(value, "%!") {
			t.Errorf("%s = %q", tc.key, value)
		}
	}
	if !strings.Contains(english["bcs2.adjust.fromHint"], "Earlier periods keep their limits") {
		t.Fatal("permanent scope lost its history promise")
	}
}
