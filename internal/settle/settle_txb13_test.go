// SPDX-License-Identifier: MIT

package settle

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNetDoesNotCancelDifferentCurrencies(t *testing.T) {
	expenses := []Expense{
		{Payer: "sam", Shares: map[string]money.Money{"lee": money.New(3000, "USD")}},
		{Payer: "lee", Shares: map[string]money.Money{"sam": money.New(3000, "EUR")}},
	}

	settlements := []Settlement{{From: "sam", To: "lee", Amount: money.New(100, "EUR")}}
	net, err := NetWithRates(expenses, settlements, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}, "USD")
	if err != nil {
		t.Fatalf("NetWithRates: %v", err)
	}
	if net["sam"].Amount != -190 || net["lee"].Amount != 190 {
		t.Fatalf("converted net = %+v, want sam -190 USD and lee +190 USD", net)
	}
}

func TestNetWithRatesRefusesUnknownCurrency(t *testing.T) {
	expenses := []Expense{{Payer: "sam", Shares: map[string]money.Money{"lee": money.New(3000, "EUR")}}}
	if _, err := NetWithRates(expenses, nil, currency.Rates{Base: "USD"}, "USD"); err == nil {
		t.Fatal("missing EUR rate should refuse to produce a settle-up result")
	}
}
