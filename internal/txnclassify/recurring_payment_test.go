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
	if got[0].TransactionID != rows[0].ID || got[0].AccountID != "priya-car" || got[0].AccountName != "Priya's Car Loan" {
		t.Fatalf("suggestion = %#v", got[0])
	}
	if rows[0].TransferAccountID != "" || rows[0].CategoryID != "" {
		t.Fatal("suggestion rewrote transaction history")
	}
}

func TestRecurringLiabilityDestinationSkipsSourceAccount(t *testing.T) {
	card := domain.Account{ID: "card", Name: "Priya Credit Card", Class: domain.ClassLiability, Currency: "USD", MinPayment: money.New(48000, "USD")}
	rule := domain.Recurring{ID: "card-payment", Label: "Priya card payment", AccountID: "card", Amount: money.New(-48000, "USD")}
	if got, ok := RecurringLiabilityDestination(rule, []domain.Account{card}); ok {
		t.Fatalf("source account was selected as its own destination: %+v", got)
	}
}

func TestFindRecurringPaymentSuggestionsSkipsNearbyRecordedTransfer(t *testing.T) {
	base := time.Date(2026, time.October, 17, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{ID: "priya-car", Name: "Priya's Car Loan", Class: domain.ClassLiability, Currency: "USD", MinPayment: money.New(48000, "USD")}}
	recurring := []domain.Recurring{{ID: "car-p", Label: "Car payment Priya", AccountID: "checking", Amount: money.New(-48000, "USD")}}
	expense := domain.Transaction{ID: "imported-expense", AccountID: "checking", Payee: "Car payment Priya", Amount: money.New(-48000, "USD"), Date: base}
	transfer := domain.Transaction{ID: "posted-transfer", AccountID: "checking", TransferAccountID: "priya-car", Amount: money.New(-48000, "USD"), Date: base.Add(4 * 24 * time.Hour)}

	if got := FindRecurringPaymentSuggestions([]domain.Transaction{expense, transfer}, accounts, recurring); len(got) != 0 {
		t.Fatalf("nearby transfer still offered a duplicate suggestion: %+v", got)
	}

	transfer.Date = base.Add(5 * 24 * time.Hour)
	got := FindRecurringPaymentSuggestions([]domain.Transaction{expense, transfer}, accounts, recurring)
	if len(got) != 1 || got[0].TransactionID != expense.ID {
		t.Fatalf("transfer outside PairWindow suppressed suggestion: %+v", got)
	}
}

func TestFindRecurringPaymentSuggestionsPreparesEachRecurringDestinationOnce(t *testing.T) {
	// Multiple expenses should reuse the prepared destination and label data.
	accounts := []domain.Account{{ID: "priya-car", Name: "Priya's Car Loan", Class: domain.ClassLiability, Currency: "USD", MinPayment: money.New(48000, "USD")}}
	recurring := []domain.Recurring{{ID: "car-p", Label: "Car payment Priya", AccountID: "checking", Amount: money.New(-48000, "USD")}}
	rows := []domain.Transaction{
		{ID: "expense-1", Payee: "Car payment Priya", Amount: money.New(-48000, "USD")},
		{ID: "expense-2", Payee: "Car payment Priya", Amount: money.New(-48000, "USD")},
	}
	got := FindRecurringPaymentSuggestions(rows, accounts, recurring)
	if len(got) != 2 {
		t.Fatalf("suggestions = %+v, want one per expense", got)
	}
}
