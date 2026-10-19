// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRecurringLiabilityDestinationRequiresUniquePaymentMatch(t *testing.T) {
	accounts := []domain.Account{
		{ID: "checking", Name: "Joint Checking", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "priya-car", Name: "Priya's Car Loan", Class: domain.ClassLiability, Currency: "USD", MinPayment: money.New(48000, "USD")},
		{ID: "marcus-car", Name: "Marcus's Car Loan", Class: domain.ClassLiability, Currency: "USD", MinPayment: money.New(62000, "USD")},
	}
	tests := []struct {
		name string
		rule domain.Recurring
		want string
	}{
		{name: "Priya sample car payment", rule: domain.Recurring{ID: "p", Label: "Car payment (Priya)", Amount: money.New(-48000, "USD")}, want: "priya-car"},
		{name: "Marcus sample car payment", rule: domain.Recurring{ID: "m", Label: "Car payment (Marcus)", Amount: money.New(-62000, "USD")}, want: "marcus-car"},
		{name: "amount mismatch", rule: domain.Recurring{ID: "x", Label: "Car payment (Priya)", Amount: money.New(-47000, "USD")}},
		{name: "currency mismatch", rule: domain.Recurring{ID: "x", Label: "Car payment (Priya)", Amount: money.New(-48000, "CAD")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := RecurringLiabilityDestination(tc.rule, accounts)
			if tc.want == "" {
				if ok {
					t.Fatalf("unexpected destination %q", got.ID)
				}
				return
			}
			if !ok || got.ID != tc.want {
				t.Fatalf("destination = %q, %v; want %q", got.ID, ok, tc.want)
			}
		})
	}
	ambiguous := append(accounts, domain.Account{ID: "another-car", Name: "Another Car Loan", Class: domain.ClassLiability, Currency: "USD", MinPayment: money.New(48000, "USD")})
	if got, ok := RecurringLiabilityDestination(domain.Recurring{ID: "x", Label: "Car payment", Amount: money.New(-48000, "USD")}, ambiguous); ok {
		t.Fatalf("ambiguous generic label resolved to %q", got.ID)
	}
}

func TestFindRecurringPaymentSuggestionsFlagsSampleExpenseWithoutRewriting(t *testing.T) {
	accounts := []domain.Account{{ID: "priya-car", Name: "Priya's Car Loan", Class: domain.ClassLiability, Currency: "USD", MinPayment: money.New(48000, "USD")}}
	recurring := []domain.Recurring{{ID: "car-p", Label: "Car payment (Priya)", Amount: money.New(-48000, "USD")}}
	rows := []domain.Transaction{
		{ID: "sample-oct-car-payment", Payee: "Car payment (Priya)", Amount: money.New(-48000, "USD"), Date: time.Date(2026, time.October, 17, 0, 0, 0, 0, time.UTC)},
		{ID: "already-transfer", Payee: "Car payment (Priya)", Amount: money.New(-48000, "USD"), TransferAccountID: "priya-car"},
		{ID: "different-amount", Payee: "Car payment (Priya)", Amount: money.New(-47000, "USD")},
	}
	got := FindRecurringPaymentSuggestions(rows, accounts, recurring)
	if len(got) != 1 {
		t.Fatalf("suggestions = %#v, want exactly the matching expense", got)
	}
	if got[0].TransactionID != rows[0].ID || got[0].AccountID != "priya-car" || got[0].Message != "Looks like a payment to Priya's Car Loan — record as a transfer?" {
		t.Fatalf("suggestion = %#v", got[0])
	}
	if rows[0].TransferAccountID != "" || rows[0].CategoryID != "" {
		t.Fatal("suggestion rewrote transaction history")
	}
}
