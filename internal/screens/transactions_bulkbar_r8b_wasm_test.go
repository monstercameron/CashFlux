// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "testing"

func TestR8BBulkBarInlineLayerAllowsPopoversAboveLedger(t *testing.T) {
	style := txnBulkBarWidgetStyle()
	if got := style["overflow"]; got != "visible" {
		t.Errorf("bulk bar widget overflow = %q, want visible so popovers escape the tile", got)
	}
	if got := style["z-index"]; got != "70" {
		t.Errorf("bulk bar widget z-index = %q, want 70 above ledger rows at 60/61", got)
	}
	if _, ok := style["overflow-x"]; ok {
		t.Error("bulk bar widget must not set horizontal scrolling")
	}
}
