// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestV5RowControlsHaveAtLeast32PixelTargets(t *testing.T) {
	resetSheet()
	registerV5Row()
	css := Build()
	for _, want := range []string{
		".bento-ledger .txn-ledger-row button, .bento-ledger .txn-ledger-row input[type=checkbox]{box-sizing:border-box;min-width:32px;min-height:32px;",
		".bento-ledger .txn-ledger-row .txn-row-menu button, .bento-ledger .txn-ledger-row .txn-row-edit{width:32px;height:32px;min-width:32px;min-height:32px;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("missing 32×32 transaction target rule %q", want)
		}
	}
}

func TestV5RowToastStacksAboveSelectedBulkBar(t *testing.T) {
	resetSheet()
	registerV5Row()
	css := Build()
	if !strings.Contains(css, "body:has(.bento-ledger.txn-has-selection) .toast{bottom:calc(60px + env(safe-area-inset-bottom, 0px));") {
		t.Fatalf("toast selected-state offset missing: %s", css)
	}
}
