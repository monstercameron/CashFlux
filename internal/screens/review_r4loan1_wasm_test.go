// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestReviewR4LOAN1SuggestionsHaveTheirOwnGroupAndActions(t *testing.T) {
	suggestion := txnclassify.PaymentSuggestion{
		TransactionID: "payment-1", AccountID: "loan-1", AccountName: "Priya's Car Loan",
	}
	var recorded, dismissed string
	f := render.New(t)
	f.Render(reviewLoanPaymentGroup(reviewLoanPaymentGroupProps{
		Suggestions: []txnclassify.PaymentSuggestion{suggestion},
		OnRecord:    func(got txnclassify.PaymentSuggestion) { recorded = got.TransactionID },
		OnDismiss:   func(id string) { dismissed = id },
	}))
	if got := byTestID(f, "div", "review-loan-payment-group"); got == nil || got.Text() == "" {
		t.Fatal("Review did not render a separate loan-payment suggestion group")
	}
	if !strings.Contains(f.Text(), "Looks like a payment to Priya's Car Loan") {
		t.Fatalf("Review copy = %q, want destination named", f.Text())
	}
	if button := byTestID(f, "button", "review-loan-payment-record"); button == nil || button.Text() != "Record as transfer" {
		t.Fatalf("record action = %v", button)
	} else {
		button.Click()
	}
	if recorded != suggestion.TransactionID {
		t.Fatalf("recorded transaction = %q, want %q", recorded, suggestion.TransactionID)
	}
	if button := byTestID(f, "button", "review-loan-payment-dismiss"); button == nil || button.Text() != "Not a loan payment" {
		t.Fatalf("dismiss action = %v", button)
	} else {
		button.Click()
	}
	if dismissed != suggestion.TransactionID {
		t.Fatalf("dismissed transaction = %q, want %q", dismissed, suggestion.TransactionID)
	}
}

func TestReviewR4LOAN1RecordConvertsTheSuggestedExpenseInPlace(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
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
	if err := app.PutRecurring(domain.Recurring{ID: "car-payment", Label: "Car payment (Priya)", Amount: money.New(-48000, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "legacy-payment", AccountID: "checking", Date: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC),
		Payee: "Car payment (Priya)", Desc: "Car payment (Priya)", Amount: money.New(-48000, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	suggestions := app.RecurringPaymentSuggestions()
	if len(suggestions) != 1 {
		t.Fatalf("suggestions = %+v, want one", suggestions)
	}
	if err := classifyRecurringLoanPayment(app, suggestions[0]); err != nil {
		t.Fatalf("classifyRecurringLoanPayment: %v", err)
	}
	got, ok := app.TransactionByID("legacy-payment")
	if !ok || got.TransferAccountID != "loan" || got.BillAccountID != "loan" {
		t.Fatalf("converted transaction = %+v, want an in-place loan transfer", got)
	}
	if len(app.Transactions()) != 1 {
		t.Fatalf("conversion created or removed transaction rows: %d, want the original one", len(app.Transactions()))
	}
}
