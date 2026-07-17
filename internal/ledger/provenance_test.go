// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func provTxn(acct string, day int, src domain.TxnSource, desc string) domain.Transaction {
	return domain.Transaction{
		ID: desc, AccountID: acct, Source: src, Desc: desc,
		Date:   time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-100, "USD"),
	}
}

func TestBalanceProvenance(t *testing.T) {
	isAdj := func(tx domain.Transaction) bool { return tx.Desc == "Balance adjustment" }

	cases := []struct {
		name string
		txns []domain.Transaction
		want ProvenanceKind
	}{
		{"no transactions", nil, ProvenanceOpening},
		{"other account only", []domain.Transaction{provTxn("other", 1, domain.TxnSourceManual, "x")}, ProvenanceOpening},
		{"newest is manual entry", []domain.Transaction{
			provTxn("a1", 1, domain.TxnSourceImported, "old import"),
			provTxn("a1", 5, domain.TxnSourceManual, "coffee"),
		}, ProvenanceDerived},
		{"newest is import", []domain.Transaction{
			provTxn("a1", 1, domain.TxnSourceManual, "coffee"),
			provTxn("a1", 5, domain.TxnSourceImported, "stmt row"),
		}, ProvenanceImported},
		{"newest is scanned", []domain.Transaction{
			provTxn("a1", 5, domain.TxnSourceScanned, "receipt"),
		}, ProvenanceImported},
		{"newest is an adjustment", []domain.Transaction{
			provTxn("a1", 1, domain.TxnSourceImported, "stmt row"),
			provTxn("a1", 5, domain.TxnSourceManual, "Balance adjustment"),
		}, ProvenanceAdjusted},
		{"same-day tie: later slice position wins", []domain.Transaction{
			provTxn("a1", 5, domain.TxnSourceManual, "coffee"),
			provTxn("a1", 5, domain.TxnSourceImported, "stmt row"),
		}, ProvenanceImported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, at := BalanceProvenance("a1", tc.txns, isAdj)
			if got != tc.want {
				t.Errorf("kind = %s, want %s", got, tc.want)
			}
			if tc.want == ProvenanceOpening && !at.IsZero() {
				t.Error("opening provenance should carry a zero time")
			}
			if tc.want != ProvenanceOpening && at.IsZero() {
				t.Error("non-opening provenance should carry the newest date")
			}
		})
	}

	// nil predicate: adjustments simply classify by source.
	got, _ := BalanceProvenance("a1", []domain.Transaction{
		provTxn("a1", 5, domain.TxnSourceManual, "Balance adjustment"),
	}, nil)
	if got != ProvenanceDerived {
		t.Errorf("nil predicate: kind = %s, want derived", got)
	}
}
