// SPDX-License-Identifier: MIT

package ui

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestFormatMoneyFig(t *testing.T) {
	tests := []struct {
		name     string
		amount   money.Money
		style    MoneyStyle
		transfer bool
		wantText string
		wantTone string
	}{
		{name: "ledger expense is unsigned and neutral", amount: money.New(-48000, "USD"), style: MoneyLedger, wantText: "$480.00", wantTone: "txn-amount-expense"},
		{name: "ledger income is signed and green", amount: money.New(123456, "USD"), style: MoneyLedger, wantText: "+$1,234.56", wantTone: "text-up"},
		{name: "transfer out is unsigned and neutral", amount: money.New(-2500, "USD"), style: MoneyLedger, transfer: true, wantText: "$25.00"},
		{name: "transfer in is unsigned and neutral", amount: money.New(2500, "USD"), style: MoneyLedger, transfer: true, wantText: "$25.00"},
		{name: "zero keeps its symbol", amount: money.Zero("USD"), style: MoneyLedger, wantText: "$0.00"},
		{name: "JPY uses zero decimals", amount: money.Zero("JPY"), style: MoneySigned, wantText: "¥0"},
		{name: "signed net is positive", amount: money.New(8500, "USD"), style: MoneySigned, wantText: "+$85.00", wantTone: "text-up"},
		{name: "signed net is negative", amount: money.New(-1200, "USD"), style: MoneySigned, wantText: "−$12.00", wantTone: "txn-amount-expense"},
		{name: "signed zero keeps its symbol", amount: money.Zero("USD"), style: MoneySigned, wantText: "$0.00"},
		{name: "minimum int formats without overflow", amount: money.New(math.MinInt64, "JPY"), style: MoneyLedger, wantText: "¥9,223,372,036,854,775,808", wantTone: "txn-amount-expense"},
		{name: "accounting negative uses parentheses", amount: money.New(-48000, "USD"), style: MoneyAccounting, wantText: "($480.00)"},
		{name: "accounting positive has no sign", amount: money.New(48000, "USD"), style: MoneyAccounting, wantText: "$480.00"},
		{name: "accounting JPY keeps zero decimals", amount: money.New(-1200, "JPY"), style: MoneyAccounting, wantText: "(¥1,200)"},
		{name: "magnitude strips sign", amount: money.New(-48000, "USD"), style: MoneyMagnitude, wantText: "$480.00"},
		{name: "unknown style safely falls back to ledger", amount: money.New(150, "USD"), style: MoneyStyle(-1), wantText: "+$1.50", wantTone: "text-up"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotTone := FormatMoneyFig(tt.amount, tt.style, tt.transfer)
			if gotText != tt.wantText || gotTone != tt.wantTone {
				t.Fatalf("FormatMoneyFig() = (%q, %q), want (%q, %q)", gotText, gotTone, tt.wantText, tt.wantTone)
			}
		})
	}
}

func TestMoneyFigAccessibleLabelSpellsDirection(t *testing.T) {
	tests := []struct {
		name     string
		amount   money.Money
		style    MoneyStyle
		transfer bool
		text     string
		want     string
	}{
		{name: "ledger expense", amount: money.New(-2500, "USD"), style: MoneyLedger, text: "$25.00", want: "Minus $25.00"},
		{name: "ledger income", amount: money.New(2500, "USD"), style: MoneyLedger, text: "+$25.00", want: "Plus $25.00"},
		{name: "transfer", amount: money.New(-2500, "USD"), style: MoneyLedger, transfer: true, text: "$25.00", want: "Transfer $25.00"},
		{name: "signed net", amount: money.New(-2500, "USD"), style: MoneySigned, text: "−$25.00", want: "Minus $25.00"},
		{name: "accounting report", amount: money.New(-2500, "USD"), style: MoneyAccounting, text: "($25.00)", want: "Minus $25.00"},
		{name: "zero", amount: money.Zero("USD"), style: MoneySigned, text: "$0.00", want: "$0.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := moneyFigAccessibleLabel(tt.amount, tt.style, tt.transfer, tt.text)
			if got != tt.want {
				t.Fatalf("moneyFigAccessibleLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
