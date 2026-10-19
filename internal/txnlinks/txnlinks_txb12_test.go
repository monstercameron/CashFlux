// SPDX-License-Identifier: MIT

package txnlinks

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRefundCandidatesExcludeAlreadyPairedRefundAndIncompatibleCurrency(t *testing.T) {
	date := day(2026, 4, 5)
	tests := []struct {
		name   string
		refund domain.Transaction
		links  []domain.TxnLink
		want   int
	}{
		{
			name:   "refund already paired",
			refund: txn("refund", "Jacket Co", 12000, date),
			links:  []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"old-purchase", "refund"}}},
			want:   0,
		},
		{
			name:   "different currencies",
			refund: domain.Transaction{ID: "refund", Payee: "Jacket Co", Amount: money.New(12000, "EUR"), Date: date},
			want:   0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			purchase := txn("purchase", "Jacket Co", -12000, day(2026, 3, 10))
			got := RefundCandidates(tc.refund, []domain.Transaction{tc.refund, purchase}, tc.links)
			if len(got) != tc.want {
				t.Fatalf("RefundCandidates returned %v, want %d candidates", ids(got), tc.want)
			}
		})
	}
}

func TestNetAdjustmentsIgnoreLegacyCrossCurrencyPair(t *testing.T) {
	original := domain.Transaction{ID: "purchase", Payee: "Jacket Co", Amount: money.New(-12000, "USD"), Date: day(2026, 3, 10)}
	refund := domain.Transaction{ID: "refund", Payee: "Jacket Co", Amount: money.New(12000, "EUR"), Date: day(2026, 4, 5)}
	link := domain.TxnLink{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}, Amount: money.New(12000, "EUR")}
	if got := NetAdjustments([]domain.Transaction{original, refund}, []domain.TxnLink{link}); len(got) != 0 {
		t.Fatalf("cross-currency pair produced net adjustments: %+v", got)
	}
}
