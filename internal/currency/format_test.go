// SPDX-License-Identifier: MIT

package currency

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestFormatInBaseSameCurrency(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{}}
	got, err := r.FormatInBase(money.New(123456, "USD"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "$1,234.56" {
		t.Errorf("FormatInBase = %q, want $1,234.56", got)
	}
}

func TestFormatInBaseNegativeParenthesized(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{}}
	got, _ := r.FormatInBase(money.New(-24055, "USD"))
	if got != "($240.55)" {
		t.Errorf("FormatInBase negative = %q, want ($240.55)", got)
	}
}

func TestFormatInBaseConverts(t *testing.T) {
	// 1 EUR = 1.10 USD; €100.00 -> $110.00.
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	got, err := r.FormatInBase(money.New(10000, "EUR"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "$110.00" {
		t.Errorf("FormatInBase(€100) = %q, want $110.00", got)
	}
}

func TestFormatAccountingTargetCurrency(t *testing.T) {
	// Format a USD amount in EUR: base USD, 1 EUR = 1.25 USD → $100 = €80.00.
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}}
	got, err := r.FormatAccounting(money.New(10000, "USD"), "EUR")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "€80.00" {
		t.Errorf("FormatAccounting in EUR = %q, want €80.00", got)
	}
}

func TestFormatAccountingMissingRate(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{}}
	if _, err := r.FormatAccounting(money.New(100, "GBP"), "USD"); err == nil {
		t.Error("expected error for missing GBP rate")
	}
}
