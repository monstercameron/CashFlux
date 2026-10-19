// SPDX-License-Identifier: MIT

package reviewview

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

func TestTCW4DismissalSurvivesSuggestionDisappearing(t *testing.T) {
	dismissed := map[string]bool{"payment": true}
	// The transaction still exists, but no longer matches the recurring amount.
	current := []domain.Transaction{{ID: "payment", Amount: money.New(-49000, "USD")}}
	got, changed := PruneDismissals(dismissed, current)
	if changed || !got["payment"] {
		t.Fatal("temporary suggestion disappearance erased the dismissal")
	}
	// Reverting the amount makes the offer return, but it remains dismissed.
	if visible := VisibleLoanPayments([]txnclassify.PaymentSuggestion{{TransactionID: "payment"}}, got); len(visible) != 0 {
		t.Fatal("dismissed suggestion returned after reverting the amount")
	}
}

func TestTCW4PruneOnlyDeletedOrTransferredDismissals(t *testing.T) {
	for _, tc := range []struct {
		name         string
		transactions []domain.Transaction
		keep         bool
	}{
		{"deleted", nil, false},
		{"transfer", []domain.Transaction{{ID: "payment", TransferAccountID: "loan"}}, false},
		{"ordinary edit", []domain.Transaction{{ID: "payment", Payee: "Edited", CategoryID: "other"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := map[string]bool{"payment": true}
			got, changed := PruneDismissals(input, tc.transactions)
			if got["payment"] != tc.keep || changed == tc.keep || !input["payment"] {
				t.Fatalf("pruned = %v, changed = %v; input = %v", got, changed, input)
			}
		})
	}
}
