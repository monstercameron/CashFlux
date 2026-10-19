// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBC6N8AutoBudgetErrorUsesBodyStyle(t *testing.T) {
	resetSheet()
	registerBC6N8()
	if !strings.Contains(Build(), ".autobudget-error{font-size:var(--fs-body);font-weight:500;}") {
		t.Fatal("auto-budget error should use the shared body scale and weight")
	}
}
