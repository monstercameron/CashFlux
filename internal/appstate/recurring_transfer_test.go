// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPostDueRecurringLiabilityPaymentCreatesTransferPair(t *testing.T) {
	a := newApp(t, false)
	for _, account := range []domain.Account{
		{ID: "checking", Name: "Joint Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(0, "USD")},
		{ID: "mortgage", Name: "Mortgage", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeMortgage, Currency: "USD", OpeningBalance: money.New(-10000000, "USD"), MinPayment: money.New(148000, "USD")},
	} {
		if err := a.PutAccount(account); err != nil {
			t.Fatalf("PutAccount %q: %v", account.ID, err)
		}
	}
	due := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	if err := a.PutRecurring(domain.Recurring{
		ID: "mortgage-payment", Label: "Mortgage payment", Amount: money.New(-148000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: due, AccountID: "checking", Autopost: true,
	}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}

	posted, err := a.PostDueRecurring(due)
	if err != nil {
		t.Fatalf("PostDueRecurring: %v", err)
	}
	if posted != 1 {
		t.Fatalf("posted count = %d, want 1 occurrence", posted)
	}
	var out, in []domain.Transaction
	for _, txn := range a.Transactions() {
		if txn.Date.Equal(due) {
			if txn.AccountID == "checking" {
				out = append(out, txn)
			}
			if txn.AccountID == "mortgage" {
				in = append(in, txn)
			}
		}
	}
	if len(out) != 1 || len(in) != 1 {
		t.Fatalf("transfer legs = %d out, %d in; want one pair", len(out), len(in))
	}
	if out[0].Amount.Amount != -148000 || out[0].TransferAccountID != "mortgage" || out[0].CategoryID != "" {
		t.Fatalf("outgoing leg = %#v", out[0])
	}
	if in[0].Amount.Amount <= 0 || in[0].TransferAccountID != "checking" || in[0].TransferGroupID != out[0].TransferGroupID {
		t.Fatalf("incoming leg = %#v", in[0])
	}
}

func TestRecordBillPaymentRecurringLiabilityCreatesTransferPair(t *testing.T) {
	a := newApp(t, false)
	for _, account := range []domain.Account{
		{ID: "checking", Name: "Joint Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(0, "USD")},
		{ID: "priya-car", Name: "Priya's Car Loan", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", OpeningBalance: money.New(-2000000, "USD"), MinPayment: money.New(48000, "USD")},
	} {
		if err := a.PutAccount(account); err != nil {
			t.Fatalf("PutAccount %q: %v", account.ID, err)
		}
	}
	if err := a.PutRecurring(domain.Recurring{
		ID: "car-p", Label: "Car payment (Priya)", Amount: money.New(-48000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, time.November, 17, 0, 0, 0, 0, time.UTC), AccountID: "checking",
	}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	if err := a.RecordBillPayment("recurring:car-p", "Car payment (Priya)", money.New(-48000, "USD")); err != nil {
		t.Fatalf("RecordBillPayment: %v", err)
	}
	if got := a.RecurringPaymentSuggestions(); len(got) != 0 {
		t.Fatalf("created transfer was suggested as an expense: %#v", got)
	}
	var legs int
	for _, txn := range a.Transactions() {
		if txn.TransferAccountID != "" && txn.Source == domain.TxnSourceRecurring {
			legs++
		}
	}
	if legs != 2 {
		t.Fatalf("recurring transfer legs = %d, want 2", legs)
	}
}
