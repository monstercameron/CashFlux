// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestSignedEditAmount(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		direction string
		currency  string
		want      int64
	}{
		{name: "expense", raw: "12.34", direction: "expense", currency: "USD", want: -1234},
		{name: "income", raw: "12.34", direction: "income", currency: "USD", want: 1234},
		{name: "negative input normalized", raw: "-12.34", direction: "income", currency: "USD", want: 1234},
		{name: "zero decimal currency", raw: "1200", direction: "expense", currency: "JPY", want: -1200},
		{name: "invalid amount", raw: "nope", direction: "income", currency: "USD", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := signedEditAmount(tt.raw, tt.direction, tt.currency); got != tt.want {
				t.Fatalf("signedEditAmount(%q, %q, %q) = %d, want %d", tt.raw, tt.direction, tt.currency, got, tt.want)
			}
		})
	}
}

func TestReceiptCountLabel(t *testing.T) {
	tests := []struct {
		count int
		key   string
	}{
		{count: 1, key: "transactions.receiptAttached"},
		{count: 3, key: "transactions.receiptsAttached"},
	}
	for _, tt := range tests {
		if got, want := receiptCountLabel(tt.count), uistate.T(tt.key, tt.count); got != want {
			t.Errorf("receiptCountLabel(%d) = %q, want %q", tt.count, got, want)
		}
	}
}
