// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR1IOpenTransactionMenuHighlightsOwningRow(t *testing.T) {
	resetSheet()
	registerTxnAudit()
	css := Build()
	for _, want := range []string{
		".txn-table tbody tr:has(.overflow-menu-open)",
		"background:color-mix(in srgb, var(--accent) 9%, var(--bg-card))",
		".txn-table tbody tr:has(.overflow-menu-open) > td",
		"background:transparent",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("transaction menu row highlight CSS missing %q", want)
		}
	}
}
