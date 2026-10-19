// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNoMatchR4LOAN1ScheduledLoanPaymentIsNeutralAndNamesDestination(t *testing.T) {
	item := bills.Bill{
		Amount: money.New(48000, "USD"), TransferAccountID: "loan", TransferAccountName: "Priya's Car Loan",
	}
	amount, tone := txnScheduledAmount(item)
	if amount != "$480.00" || tone != "" || strings.Contains(amount, "−") {
		t.Fatalf("scheduled transfer amount = (%q, %q), want unsigned neutral amount", amount, tone)
	}
	if got := scheduledTransferCategory(item.TransferAccountName); got != "Transfer → Priya's Car Loan" {
		t.Fatalf("scheduled transfer category = %q", got)
	}
}

func TestNoMatchR4LOAN1ScheduledCategoryNamesLiabilityAndKeepsSourceAccount(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []domain.Account{
		{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"},
		{ID: "loan", Name: "Priya's Car Loan", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", MinPayment: money.New(48000, "USD")},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatalf("PutAccount: %v", err)
		}
	}
	if err := app.PutRecurring(domain.Recurring{
		ID: "car-payment", Label: "Car payment (Priya)", Amount: money.New(-48000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AccountID: "checking",
	}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	item := bills.Bill{
		AccountID: "recurring:car-payment", Amount: money.New(48000, "USD"),
		TransferAccountID: "loan", TransferAccountName: "Priya's Car Loan",
	}
	category, source := txnScheduledRowDetails(app, item, map[string]string{"checking": "Checking", "loan": "Priya's Car Loan"}, nil)
	if category != "Transfer → Priya's Car Loan" || source != "Checking" {
		t.Fatalf("scheduled row details = (%q, %q), want destination category and source account", category, source)
	}
}

func TestFUT21FutureTotalExcludesScheduledTransfers(t *testing.T) {
	items := []bills.Bill{
		{Amount: money.New(1000, "USD")},
		{Amount: money.New(2500, "EUR"), TransferAccountID: "loan"},
		{Amount: money.New(2000, "EUR")},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}}
	total, unconverted := txnFutureTotal(items, rates, "USD")
	if total.Amount != 4000 || total.Currency != "USD" || unconverted != 0 {
		t.Fatalf("future due total = %+v, unconverted %d; want 4000 USD, 0 unconverted", total, unconverted)
	}
}
