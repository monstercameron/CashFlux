// SPDX-License-Identifier: MIT

package domain

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTxnTemplateSignedMinorMinInt64Safe(t *testing.T) {
	inc := TxnTemplate{AmountMinor: math.MinInt64, Direction: DirectionIncome}
	if got := inc.SignedMinor(); got != math.MaxInt64 {
		t.Fatalf("income SignedMinor(MinInt64) = %d, want MaxInt64", got)
	}
	exp := TxnTemplate{AmountMinor: math.MinInt64, Direction: DirectionExpense}
	if got := exp.SignedMinor(); got != math.MinInt64 {
		t.Fatalf("expense SignedMinor(MinInt64) = %d, want MinInt64", got)
	}
}

func TestSharedExpenseTotalIgnoresMixedCurrencyAndSaturates(t *testing.T) {
	e := SharedExpense{
		Shares: []SharedExpenseShare{
			{MemberID: "m1", Amount: money.New(500, "USD")},
			{MemberID: "m2", Amount: money.New(999, "EUR")},
			{MemberID: "m3", Amount: money.New(300, "usd")},
		},
	}
	if got := e.Total(); got.Amount != 800 || got.Currency != "USD" {
		t.Fatalf("Total = %+v, want 800 USD", got)
	}
	ov := SharedExpense{
		Shares: []SharedExpenseShare{
			{MemberID: "m1", Amount: money.New(math.MaxInt64, "USD")},
			{MemberID: "m2", Amount: money.New(10, "USD")},
		},
	}
	if got := ov.Total(); got.Amount != math.MaxInt64 {
		t.Fatalf("overflow Total = %d, want MaxInt64", got.Amount)
	}
}
