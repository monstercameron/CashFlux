// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestBQ10ACopyHasVisibleFailureStates(t *testing.T) {
	for _, key := range []string{"budgets.autoError", "coverAll.currencyError", "sweep.error"} {
		text, ok := english[key]
		if !ok || strings.TrimSpace(text) == "" || strings.Contains(text, key) {
			t.Errorf("English[%q] = %q, want readable copy", key, text)
		}
	}
}
