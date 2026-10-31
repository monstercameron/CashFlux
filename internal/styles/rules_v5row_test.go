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
		".bento-ledger .txn-ledger-row button:not(.btn-tool){box-sizing:border-box;min-width:32px;min-height:32px;",
		".bento-ledger .txn-ledger-row .txn-row-menu [role=menuitem]{width:100%;min-width:160px;min-height:32px;}",
		".bento-ledger .txn-ledger-row input[type=checkbox]{box-sizing:border-box;width:16px;height:16px;min-width:16px;min-height:16px;padding:0;appearance:none;",
		".bento-ledger .txn-ledger-row input[type=checkbox]:checked{background:var(--accent);border-color:var(--accent);}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("missing 32×32 transaction target rule %q", want)
		}
	}
	if strings.Contains(css, ".txn-row-menu button{width:32px;") {
		t.Fatal("menu items must not inherit the 32px trigger width")
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
