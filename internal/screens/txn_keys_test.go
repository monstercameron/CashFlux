// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTxnActionForKey(t *testing.T) {
	tests := []struct {
		key  string
		want txnKeyAction
	}{
		{key: "/", want: txnKeySearch},
		{key: "j", want: txnKeyNext},
		{key: "k", want: txnKeyPrevious},
		{key: "x", want: txnKeySelect},
		{key: "e", want: txnKeyEdit},
		{key: "Enter", want: txnKeyEdit},
		{key: "c", want: txnKeyCategory},
		{key: "r", want: txnKeyReview},
		{key: "?", want: txnKeyHelp},
		{key: "Escape", want: txnKeyNone},
		{key: "J", want: txnKeyNone},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := txnActionForKey(tt.key); got != tt.want {
				t.Fatalf("txnActionForKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestTxnShortcutBlocked(t *testing.T) {
	tests := []struct {
		name        string
		editable    bool
		dialogOpen  bool
		wantBlocked bool
	}{
		{name: "ordinary ledger focus"},
		{name: "input focused", editable: true, wantBlocked: true},
		{name: "dialog open", dialogOpen: true, wantBlocked: true},
		{name: "input and dialog", editable: true, dialogOpen: true, wantBlocked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnShortcutBlocked(tt.editable, tt.dialogOpen); got != tt.wantBlocked {
				t.Fatalf("txnShortcutBlocked(%t, %t) = %t, want %t", tt.editable, tt.dialogOpen, got, tt.wantBlocked)
			}
		})
	}
}

func TestTxnDialogConsumesShortcutsWithoutRunningThem(t *testing.T) {
	tests := []struct {
		name       string
		editable   bool
		dialogOpen bool
		key        string
		want       bool
	}{
		{name: "dialog consumes help key", dialogOpen: true, key: "?", want: true},
		{name: "dialog consumes ledger key", dialogOpen: true, key: "j", want: true},
		{name: "input keeps typed question mark", editable: true, dialogOpen: true, key: "?"},
		{name: "unrelated key is not consumed", dialogOpen: true, key: "z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnShortcutConsumedWithoutAction(tt.editable, tt.dialogOpen, tt.key); got != tt.want {
				t.Fatalf("txnShortcutConsumedWithoutAction(%t, %t, %q) = %t, want %t", tt.editable, tt.dialogOpen, tt.key, got, tt.want)
			}
		})
	}
}

func TestTxnReviewed(t *testing.T) {
	tests := []struct {
		name       string
		input      domain.Transaction
		want       bool
		wantReview bool
	}{
		{name: "unreviewed transaction", input: domain.Transaction{ID: "txn-1"}, want: true, wantReview: true},
		{name: "already reviewed transaction", input: domain.Transaction{ID: "txn-2", Reviewed: true}, want: false, wantReview: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := txnReviewed(tt.input)
			if changed != tt.want || got.Reviewed != tt.wantReview {
				t.Fatalf("txnReviewed() = (%t, %t), want (changed=%t, reviewed=%t)", changed, got.Reviewed, tt.want, tt.wantReview)
			}
		})
	}
}
