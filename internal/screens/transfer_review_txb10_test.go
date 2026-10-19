// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

func TestTransferReviewStableKeysDistinguishCurrencies(t *testing.T) {
	cases := []struct {
		name string
		a    txnclassify.Batch
		b    txnclassify.Batch
	}{
		{
			name: "same counterparty",
			a:    txnclassify.Batch{CounterpartyID: "savings", Currency: "USD"},
			b:    txnclassify.Batch{CounterpartyID: "savings", Currency: "EUR"},
		},
		{
			name: "unrouted",
			a:    txnclassify.Batch{Currency: "USD"},
			b:    txnclassify.Batch{Currency: "EUR"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotA, gotB := transferReviewTestKey(tc.a), transferReviewTestKey(tc.b)
			if gotA == gotB {
				t.Fatalf("stable keys collide across currencies: %q", gotA)
			}
			for _, prefix := range []string{"txnreview-batch-", "txnreview-select-", "txnreview-account-"} {
				idA, idB := prefix+gotA, prefix+gotB
				if idA == idB {
					t.Errorf("%s test IDs collide: %q", prefix, idA)
				}
			}
		})
	}
}
