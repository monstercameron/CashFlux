// SPDX-License-Identifier: MIT

package txnfilter

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestTextSearchMatchesCleanedPayee verifies that when a Labels.Payee resolver is
// supplied, a text search matches the CLEANED merchant name (the TX1/SM-1 alias
// display) — not just the raw payee/desc — so renamed merchants surface when you
// search the clean name shown in the ledger. Without a resolver the raw fields still
// match and the cleaned name does not (the pure default).
func TestTextSearchMatchesCleanedPayee(t *testing.T) {
	txn := domain.Transaction{
		ID:     "t1",
		Payee:  "BEACON BANK HOME LOANS 800-555",
		Desc:   "Mortgage payment",
		Amount: money.New(-180000, "USD"),
	}
	txns := []domain.Transaction{txn}

	// A resolver that cleans the raw merchant string to a tidy display name.
	resolve := func(raw string) string {
		if raw == "BEACON BANK HOME LOANS 800-555" {
			return "Beacon Mortgage"
		}
		return raw
	}

	cases := []struct {
		name    string
		query   string
		payee   func(string) string
		wantHit bool
	}{
		{"raw payee still matches without resolver", "beacon bank", nil, true},
		{"desc still matches without resolver", "mortgage", nil, true},
		{"clean name misses without resolver", "beacon mortgage", nil, false},
		{"clean name matches with resolver", "beacon mortgage", resolve, true},
		{"partial clean name matches with resolver", "mortgage", resolve, true},
		{"unrelated query never matches", "netflix", resolve, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyWithLabels(txns, Criteria{Text: tc.query}, Labels{Payee: tc.payee})
			if (len(got) == 1) != tc.wantHit {
				t.Fatalf("query %q (resolver=%v): got %d matches, want hit=%v",
					tc.query, tc.payee != nil, len(got), tc.wantHit)
			}
		})
	}
}
