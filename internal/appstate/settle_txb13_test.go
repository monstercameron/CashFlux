// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestSettleUpCheckedConvertsRecordedCurrencies(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutSettings(store.Settings{BaseCurrency: "EUR", FXRates: map[string]float64{"USD": 0.9}}); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	if err := a.PutSharedExpense(domain.SharedExpense{
		ID: "usd-expense", PayerID: "sam",
		Shares: []domain.SharedExpenseShare{{MemberID: "lee", Amount: money.New(1000, "USD")}},
	}); err != nil {
		t.Fatalf("PutSharedExpense USD: %v", err)
	}
	if err := a.PutSharedExpense(domain.SharedExpense{
		ID: "eur-expense", PayerID: "lee",
		Shares: []domain.SharedExpenseShare{{MemberID: "sam", Amount: money.New(1000, "EUR")}},
	}); err != nil {
		t.Fatalf("PutSharedExpense EUR: %v", err)
	}

	net, _, err := a.SettleUpChecked("EUR")
	if err != nil {
		t.Fatalf("SettleUpChecked: %v", err)
	}
	if net["sam"].Amount != -100 || net["lee"].Amount != 100 {
		t.Fatalf("settle-up net = %+v, want sam -100 EUR and lee +100 EUR", net)
	}
}

func TestSettleUpCheckedRefusesMissingRate(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutSettings(store.Settings{BaseCurrency: "EUR"}); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	if err := a.PutSharedExpense(domain.SharedExpense{
		ID: "usd-expense", PayerID: "sam",
		Shares: []domain.SharedExpenseShare{{MemberID: "lee", Amount: money.New(1000, "USD")}},
	}); err != nil {
		t.Fatalf("PutSharedExpense: %v", err)
	}
	if _, _, err := a.SettleUpChecked("EUR"); err == nil {
		t.Fatal("missing USD rate should refuse to produce settle-up balances")
	}
}
