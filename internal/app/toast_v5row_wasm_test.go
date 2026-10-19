// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "testing"

func TestV5RowToastCopyNamesSavedTransactionAndBalance(t *testing.T) {
	main, balance := toastV5RowCopy("Saved — Checking is now $2,140.12.")
	if main != "Transaction saved" {
		t.Fatalf("main toast = %q", main)
	}
	if balance != "Checking balance $2,140.12" {
		t.Fatalf("balance line = %q", balance)
	}
	main, balance = toastV5RowCopy("Transaction updated.")
	if main != "Transaction saved" || balance != "" {
		t.Fatalf("updated toast = %q / %q", main, balance)
	}
}

func TestV5RowToastTimerCountsOnlyVisibleUnfocusedTime(t *testing.T) {
	tests := []struct {
		name     string
		visible  string
		hovered  bool
		focused  bool
		wantTick bool
	}{
		{name: "visible", visible: "visible", wantTick: true},
		{name: "hidden tab", visible: "hidden"},
		{name: "hovered", visible: "visible", hovered: true},
		{name: "focused", visible: "visible", focused: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toastV5RowCountsTime(tt.visible, tt.hovered, tt.focused); got != tt.wantTick {
				t.Fatalf("counts time = %v, want %v", got, tt.wantTick)
			}
		})
	}
}
