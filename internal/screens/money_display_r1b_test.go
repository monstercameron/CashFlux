// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestFmtLedgerAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   money.Money
		transfer bool
		wantText string
		wantTone string
	}{
		{name: "expense", amount: money.New(-48000, "USD"), wantText: "$480.00", wantTone: "txn-amount-expense"},
		{name: "income", amount: money.New(123456, "USD"), wantText: "+$1,234.56", wantTone: "text-up"},
		{name: "transfer out", amount: money.New(-2500, "USD"), transfer: true, wantText: "$25.00"},
		{name: "transfer in", amount: money.New(2500, "USD"), transfer: true, wantText: "$25.00"},
		{name: "zero", amount: money.Zero("USD"), wantText: "$0.00", wantTone: ""},
		{name: "minimum int", amount: money.New(math.MinInt64, "JPY"), wantText: "¥9,223,372,036,854,775,808", wantTone: "txn-amount-expense"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotTone := fmtLedgerAmount(tt.amount, tt.transfer)
			if gotText != tt.wantText || gotTone != tt.wantTone {
				t.Fatalf("fmtLedgerAmount() = (%q, %q), want (%q, %q)", gotText, gotTone, tt.wantText, tt.wantTone)
			}
		})
	}
}

func TestFmtSignedMoneyIncludesCurrencyForZero(t *testing.T) {
	tests := []struct {
		name   string
		amount money.Money
		want   string
	}{
		{name: "positive", amount: money.New(8500, "USD"), want: "+$85.00"},
		{name: "negative", amount: money.New(-1200, "USD"), want: "−$12.00"},
		{name: "zero", amount: money.Zero("USD"), want: "$0.00"},
		{name: "zero decimal currency", amount: money.Zero("JPY"), want: "¥0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmtTxnSignedMoney(tt.amount); got != tt.want {
				t.Fatalf("fmtTxnSignedMoney() = %q, want %q", got, tt.want)
			}
		})
	}
}
