// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "testing"

func TestR3KRowMenuStaysWithinEightAndKeepsRequiredActions(t *testing.T) {
	items := txnRowMenuItems(txnFrameRowProps{
		ID: "txn-1", AutoCategory: true,
		OnConfirmCategory: func(string) {},
		OnOpenSplit:       func(string) {},
		OnOpenLink:        func(string, string) {},
		OnToggleExclude:   func(string) {},
		OnDelete:          func(string) {},
	})
	if len(items) > 8 {
		t.Fatalf("richest consolidated menu has %d items, want at most 8", len(items))
	}
	want := []string{"txn-confirm-category", "txn-split-open", "txn-link-open", "txn-toggle-exclude", "txn-delete"}
	if len(items) != len(want) {
		t.Fatalf("richest consolidated menu has %d items, want %d", len(items), len(want))
	}
	for i, item := range items {
		if item.TestID != want[i] {
			t.Errorf("menu item %d = %q, want %q", i, item.TestID, want[i])
		}
	}
	if items[len(items)-1].TestID != "txn-delete" {
		t.Errorf("last menu item = %q, want Delete", items[len(items)-1].TestID)
	}
}
